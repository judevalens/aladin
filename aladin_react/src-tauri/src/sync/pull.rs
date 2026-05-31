use crate::{
    api::sync::SyncApiClient,
    db::{repo::nodes, Db, DbError, DbResult},
    events::DataEventHub,
    sync::{engine, SyncConfig},
};

// Data-layer R1-C (client) — the pull orchestrator (the cursor-advancing path).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md (§4, §6).
//
// Reads the cursor, pulls frames since it, applies each frame atomically via the
// generic engine under the per-entity seq guard, advances the cursor to the
// returned horizon (in the same txn as the last frame), and emits one UI event
// per touched entity. Pull is the ONLY path that advances the durable cursor;
// live applies without advancing. Idempotent: re-pulling is a no-op (seq guard).
// `snapshot` and `delta` apply identically — both are frames under the guard
// (a single per-user source + tombstones converge with no delete-not-present pass).

/// Pulls and applies all available frames, returning the number of entities
/// applied. Loops until the cursor stops advancing.
pub fn pull_and_apply(
    db: &Db,
    events: &DataEventHub,
    config: &SyncConfig,
    api: &dyn SyncApiClient,
) -> DbResult<usize> {
    if !config_ready(config) {
        return Ok(0);
    }

    let registry = engine::Registry::tree();
    let mut total = 0usize;
    loop {
        let cursor = db.with_conn(nodes::get_cursor)?;
        let result = api.pull(config, cursor).map_err(DbError::Api)?;

        let has_frames = result.frames.iter().any(|f| !f.entities.is_empty());
        if !has_frames {
            // Nothing to apply; still advance the cursor if the horizon moved.
            if result.cursor > cursor {
                db.with_conn(|conn| nodes::set_cursor(conn, result.cursor))?;
            }
            break;
        }

        // Apply all frames + advance the cursor atomically, then derive one
        // event per touched entity from its final committed state. A `snapshot`
        // applies as REPLACE (doc §6): after applying, remove any local row NOT
        // in the snapshot's id set (stale cache garbage); a `delta` is a merge.
        let is_snapshot = result.mode == "snapshot";
        let (emit, applied) = db.with_tx(|tx| {
            let mut touched: Vec<String> = Vec::new();
            let mut snapshot_ids: Vec<String> = Vec::new();
            for frame in &result.frames {
                if is_snapshot {
                    for e in &frame.entities {
                        snapshot_ids.push(e.entity_id.clone());
                    }
                }
                for id in engine::apply_frame(tx, &registry, frame)? {
                    if !touched.iter().any(|t| t == &id) {
                        touched.push(id);
                    }
                }
            }
            if is_snapshot {
                // REPLACE: drop local rows the authoritative snapshot omits.
                for id in nodes::retain_only(tx, &snapshot_ids)? {
                    if !touched.iter().any(|t| t == &id) {
                        touched.push(id);
                    }
                }
            }
            let applied = touched.len();
            nodes::set_cursor(tx, result.cursor)?;
            let emit = engine::derive_events(tx, touched)?;
            Ok((emit, applied))
        })?;

        for event in emit {
            events.emit(event);
        }
        total += applied;

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
    use crate::api::sync::{Frame, FrameEntity, SyncPullResult};
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

    fn upsert(id: &str, seq: u64, title: &str) -> FrameEntity {
        FrameEntity {
            entity_kind: "folder".to_string(),
            entity_id: id.to_string(),
            seq,
            op: "upsert".to_string(),
            data: Some(serde_json::json!({
                "id": id, "kind": "folder", "parentId": null, "position": 1, "title": title
            })),
        }
    }

    struct ScriptedApi {
        responses: Mutex<Vec<SyncPullResult>>,
        calls: Mutex<Vec<u64>>,
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
        fn pull(&self, _config: &SyncConfig, since: u64) -> ApiResult<SyncPullResult> {
            self.calls.lock().unwrap().push(since);
            let mut responses = self.responses.lock().unwrap();
            if responses.is_empty() {
                Ok(SyncPullResult {
                    frames: vec![],
                    cursor: since,
                    mode: "delta".to_string(),
                })
            } else {
                Ok(responses.remove(0))
            }
        }
    }

    #[test]
    fn applies_frames_advances_cursor_and_is_idempotent() {
        let db = test_db("pull_applies");
        let api = ScriptedApi::new(vec![SyncPullResult {
            frames: vec![Frame {
                entities: vec![upsert("f1", 1, "Docs"), upsert("f1", 2, "Docs2")],
            }],
            cursor: 7,
            mode: "snapshot".to_string(),
        }]);
        let events = DataEventHub::default();

        let applied = pull_and_apply(&db, &events, &config(), &api).unwrap();
        assert_eq!(applied, 1); // one touched entity (f1)
        db.with_conn(|c| {
            assert_eq!(nodes::get_cursor(c)?, 7);
            assert_eq!(
                nodes::get_node(c, "f1")?.unwrap().title.as_deref(),
                Some("Docs2")
            );
            Ok(())
        })
        .unwrap();
        assert_eq!(&*api.calls.lock().unwrap(), &[0, 7]);

        // Re-pull from the advanced cursor: nothing new.
        let applied2 = pull_and_apply(&db, &events, &config(), &api).unwrap();
        assert_eq!(applied2, 0);
    }

    #[test]
    fn snapshot_replaces_removing_omitted_rows() {
        let db = test_db("pull_snapshot_replace");
        let events = DataEventHub::default();
        // First, a delta establishes f1 + f2 in the cache.
        let api = ScriptedApi::new(vec![SyncPullResult {
            frames: vec![Frame {
                entities: vec![upsert("f1", 1, "One"), upsert("f2", 1, "Two")],
            }],
            cursor: 5,
            mode: "delta".to_string(),
        }]);
        pull_and_apply(&db, &events, &config(), &api).unwrap();
        db.with_conn(|c| {
            assert!(nodes::get_node(c, "f1")?.is_some());
            assert!(nodes::get_node(c, "f2")?.is_some());
            Ok(())
        })
        .unwrap();

        // A cold-start snapshot that includes ONLY f1 (+ a new f3) must REPLACE:
        // f2 (omitted) is removed; f1 kept; f3 added. (doc §6)
        let api2 = ScriptedApi::new(vec![SyncPullResult {
            frames: vec![Frame {
                entities: vec![upsert("f1", 9, "One!"), upsert("f3", 9, "Three")],
            }],
            cursor: 20,
            mode: "snapshot".to_string(),
        }]);
        pull_and_apply(&db, &events, &config(), &api2).unwrap();
        db.with_conn(|c| {
            assert!(nodes::get_node(c, "f1")?.is_some(), "f1 kept");
            assert!(nodes::get_node(c, "f3")?.is_some(), "f3 added");
            assert!(nodes::get_node(c, "f2")?.is_none(), "f2 removed by REPLACE");
            assert_eq!(nodes::get_cursor(c)?, 20);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn empty_pull_is_noop() {
        let db = test_db("pull_empty");
        let api = ScriptedApi::new(vec![]);
        let events = DataEventHub::default();
        assert_eq!(pull_and_apply(&db, &events, &config(), &api).unwrap(), 0);
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

    struct FailingApi;
    impl SyncApiClient for FailingApi {
        fn pull(&self, _config: &SyncConfig, _since: u64) -> ApiResult<SyncPullResult> {
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
