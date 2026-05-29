use std::collections::HashSet;

use crate::{
    api::sync::SyncApiClient,
    db::{
        repo::nodes::{self, ApplyOutcome},
        Db, DbError, DbResult,
    },
    events::{DataEvent, DataEventHub, EntityDeletedEvent},
    sync::SyncConfig,
};

// Data-layer redesign, Phase A (client) — the read/convergence orchestrator.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// Reads the persisted cursor, pulls the coalesced delta, applies it into
// `nodes` under the per-field seq guard in a single transaction, advances the
// cursor, then emits one node-level event per touched entity reflecting its
// final committed state. Idempotent and convergent: safe to run on boot, on a
// timer, and on a realtime poke. Re-running with no new changes is a no-op.

/// Pulls and applies all available deltas, returning the number of feed changes
/// processed. Loops until the cursor stops advancing (handles future
/// pagination; today the server returns the whole coalesced delta at once).
pub fn pull_and_apply(
    db: &Db,
    events: &DataEventHub,
    config: &SyncConfig,
    api: &dyn SyncApiClient,
) -> DbResult<usize> {
    if !config_ready(config) {
        return Ok(0);
    }

    let mut total = 0usize;
    loop {
        let cursor = db.with_conn(nodes::get_cursor)?;
        let result = api.pull(config, cursor).map_err(DbError::Api)?;

        if result.changes.is_empty() {
            if result.cursor > cursor {
                db.with_conn(|conn| nodes::set_cursor(conn, result.cursor))?;
            }
            break;
        }

        // Apply the whole delta atomically, then derive one event per touched
        // entity from its final committed state (deletes win over upserts).
        let emit = db.with_tx(|tx| {
            let mut touched: Vec<String> = Vec::new();
            let mut seen: HashSet<String> = HashSet::new();
            for change in &result.changes {
                let id = match nodes::apply_change(tx, change)? {
                    ApplyOutcome::Upserted(id) | ApplyOutcome::Deleted(id) => id,
                    ApplyOutcome::Skipped => continue,
                };
                if seen.insert(id.clone()) {
                    touched.push(id);
                }
            }
            nodes::set_cursor(tx, result.cursor)?;

            let mut emit: Vec<DataEvent> = Vec::with_capacity(touched.len());
            for id in touched {
                match nodes::get_node(tx, &id)? {
                    Some(row) => emit.push(DataEvent::NodeUpserted(row)),
                    None => emit.push(DataEvent::NodeDeleted(EntityDeletedEvent { id })),
                }
            }
            Ok(emit)
        })?;

        for event in emit {
            events.emit(event);
        }
        total += result.changes.len();

        if result.cursor <= cursor {
            break;
        }
    }

    Ok(total)
}

fn config_ready(config: &SyncConfig) -> bool {
    !config.api_base_url.trim().is_empty() && config.token.as_deref().unwrap_or("").trim() != ""
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::sync::{SyncChange, SyncPullResult};
    use crate::api::{ApiError, ApiResult};
    use std::path::PathBuf;
    use std::sync::Mutex;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn test_db(name: &str) -> Db {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let path = std::env::temp_dir().join(format!(
            "aladin_pull_test_{name}_{}_{}.sqlite",
            std::process::id(),
            nanos
        ));
        Db::open(PathBuf::from(path)).unwrap()
    }

    fn config() -> SyncConfig {
        SyncConfig {
            api_base_url: "http://localhost:3000".to_string(),
            token: Some("token".to_string()),
        }
    }

    fn update(id: &str, field: &str, value: serde_json::Value, seq: i64) -> SyncChange {
        SyncChange {
            seq,
            entity_kind: "folder".to_string(),
            entity_id: id.to_string(),
            op: "update".to_string(),
            field: Some(field.to_string()),
            value: Some(value),
            mutation_id: None,
        }
    }

    fn create(id: &str, seq: i64) -> SyncChange {
        SyncChange {
            seq,
            entity_kind: "folder".to_string(),
            entity_id: id.to_string(),
            op: "create".to_string(),
            field: None,
            value: None,
            mutation_id: None,
        }
    }

    /// Serves a scripted sequence of pull responses, recording the `since`
    /// cursor it was called with each time.
    struct ScriptedApi {
        responses: Mutex<Vec<SyncPullResult>>,
        calls: Mutex<Vec<i64>>,
    }

    impl ScriptedApi {
        fn new(responses: Vec<SyncPullResult>) -> Self {
            Self {
                responses: Mutex::new(responses),
                calls: Mutex::new(Vec::new()),
            }
        }
    }

    impl SyncApiClient for ScriptedApi {
        fn pull(&self, _config: &SyncConfig, since: i64) -> ApiResult<SyncPullResult> {
            self.calls.lock().unwrap().push(since);
            let mut responses = self.responses.lock().unwrap();
            if responses.is_empty() {
                Ok(SyncPullResult {
                    changes: vec![],
                    cursor: since,
                })
            } else {
                Ok(responses.remove(0))
            }
        }

        fn push(
            &self,
            _config: &SyncConfig,
            _mutations: &[crate::api::sync::PushMutation],
        ) -> ApiResult<Vec<crate::api::sync::PushResult>> {
            Ok(vec![])
        }
    }

    #[test]
    fn applies_delta_advances_cursor_and_is_idempotent() {
        let db = test_db("pull_applies");
        let api = ScriptedApi::new(vec![SyncPullResult {
            changes: vec![
                create("f1", 1),
                update("f1", "title", serde_json::json!("Docs"), 2),
            ],
            cursor: 2,
        }]);
        let events = DataEventHub::default();

        let applied = pull_and_apply(&db, &events, &config(), &api).unwrap();
        assert_eq!(applied, 2);
        db.with_conn(|c| {
            assert_eq!(nodes::get_cursor(c)?, 2);
            assert_eq!(nodes::get_node(c, "f1")?.unwrap().title.as_deref(), Some("Docs"));
            Ok(())
        })
        .unwrap();

        // First pull used since=0, then a follow-up pull used since=2 (empty).
        assert_eq!(&*api.calls.lock().unwrap(), &[0, 2]);

        // Re-pulling from the advanced cursor returns nothing and changes nothing.
        let applied2 = pull_and_apply(&db, &events, &config(), &api).unwrap();
        assert_eq!(applied2, 0);
        db.with_conn(|c| {
            assert_eq!(nodes::get_cursor(c)?, 2);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn empty_pull_is_noop() {
        let db = test_db("pull_empty");
        let api = ScriptedApi::new(vec![]);
        let events = DataEventHub::default();
        let applied = pull_and_apply(&db, &events, &config(), &api).unwrap();
        assert_eq!(applied, 0);
        db.with_conn(|c| {
            assert_eq!(nodes::get_cursor(c)?, 0);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn unconfigured_session_skips_pull() {
        let db = test_db("pull_unconfigured");
        let api = ScriptedApi::new(vec![]);
        let events = DataEventHub::default();
        let blank = SyncConfig {
            api_base_url: "".to_string(),
            token: None,
        };
        assert_eq!(pull_and_apply(&db, &events, &blank, &api).unwrap(), 0);
        assert!(api.calls.lock().unwrap().is_empty());
    }

    // Surfaces a transport error so callers can keep the cursor and retry.
    struct FailingApi;
    impl SyncApiClient for FailingApi {
        fn pull(&self, _config: &SyncConfig, _since: i64) -> ApiResult<SyncPullResult> {
            Err(ApiError::for_test(crate::api::ApiErrorKind::Transient))
        }

        fn push(
            &self,
            _config: &SyncConfig,
            _mutations: &[crate::api::sync::PushMutation],
        ) -> ApiResult<Vec<crate::api::sync::PushResult>> {
            Err(ApiError::for_test(crate::api::ApiErrorKind::Transient))
        }
    }

    #[test]
    fn transport_error_preserves_cursor() {
        let db = test_db("pull_error");
        let events = DataEventHub::default();
        let result = pull_and_apply(&db, &events, &config(), &FailingApi);
        assert!(matches!(result, Err(DbError::Api(_))));
        db.with_conn(|c| {
            assert_eq!(nodes::get_cursor(c)?, 0);
            Ok(())
        })
        .unwrap();
    }
}
