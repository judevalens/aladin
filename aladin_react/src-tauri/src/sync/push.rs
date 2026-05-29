use crate::{
    api::sync::{PushMutation, SyncApiClient},
    db::{
        repo::{
            intent::{self, IntentOp, PendingIntent},
            nodes::{self, NodeRow},
        },
        Db, DbError, DbResult,
    },
    events::DataEventHub,
    sync::{pull, SyncConfig},
};

// Data-layer redesign, Phase B (client) — the intent sender.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// Drains pending intents in local_seq order and pushes each as ONE mutation to
// POST /api/sync/push, reading the current value from `nodes` at send time (so
// coalesced/typed-then-changed values never ship). Serial single-mutation
// sends keep the server's per-client high-water dedup correct. Resolution is a
// compare-and-set on the intent version (a concurrent re-enqueue makes the
// stale resolve no-op and the newer intent re-send):
//   - applied | duplicate  -> resolve (clear iff unchanged)
//   - error (server reject) -> drop the intent + reconcile via pull (the
//                              optimistic local write may be wrong)
//   - transport error       -> stop; leave PENDING; retry next tick
// A crashed send leaves the intent PENDING and re-sends with the same
// clientWriteId, which the server dedups.

#[derive(Debug, Default, PartialEq)]
pub struct PushStats {
    pub sent: usize,
    pub rejected: usize,
}

/// Drains and pushes all pending intents, then reconciles via pull if any were
/// rejected. Returns counts. Stops early (Err) on a transport failure so the
/// remaining intents stay PENDING for the next tick.
pub fn push_pending(
    db: &Db,
    events: &DataEventHub,
    config: &SyncConfig,
    api: &dyn SyncApiClient,
) -> DbResult<PushStats> {
    if !config_ready(config) {
        return Ok(PushStats::default());
    }

    let pending = db.with_conn(intent::list_pending)?;
    let mut stats = PushStats::default();
    let mut needs_reconcile = false;

    for item in &pending {
        let Some(mutation) = build_mutation(db, item)? else {
            // The node is gone (e.g. deleted after enqueue) — drop the stale intent.
            db.with_conn(|c| drop_intent(c, item))?;
            continue;
        };

        let results = api
            .push(config, std::slice::from_ref(&mutation))
            .map_err(DbError::Api)?; // transport error: stop, retry next tick

        let status = results.first().map(|r| r.status.as_str()).unwrap_or("error");
        match status {
            "applied" | "duplicate" => {
                db.with_conn(|c| resolve_intent(c, item))?;
                stats.sent += 1;
            }
            _ => {
                // Server reject: drop the intent; pull will heal local state.
                db.with_conn(|c| drop_intent(c, item))?;
                stats.rejected += 1;
                needs_reconcile = true;
            }
        }
    }

    if needs_reconcile {
        let _ = pull::pull_and_apply(db, events, config, api);
    }
    Ok(stats)
}

fn build_mutation(db: &Db, item: &PendingIntent) -> DbResult<Option<PushMutation>> {
    let base = |op: &str| PushMutation {
        mutation_id: item.client_write_id.clone(),
        op: op.to_string(),
        entity_kind: item.entity_kind.clone(),
        entity_id: item.entity_id.clone(),
        field: None,
        value: None,
        parent_id: None,
        position: None,
        title: None,
        artifact_type: None,
        content: None,
        summary: None,
        source_url: None,
    };

    match &item.op {
        IntentOp::Delete => Ok(Some(base("delete"))),
        IntentOp::Create => {
            let Some(node) = db.with_conn(|c| nodes::get_node(c, &item.entity_id))? else {
                return Ok(None);
            };
            let is_artifact = node.kind.eq_ignore_ascii_case("artifact");
            let mut m = base("create");
            m.parent_id = node.parent_id.clone();
            m.position = Some(node.position);
            m.title = node.title.clone();
            if is_artifact {
                m.artifact_type = node.artifact_type.clone();
                m.content = node.content.clone();
                m.summary = node.summary.clone();
                m.source_url = node.source_url.clone();
            }
            Ok(Some(m))
        }
        IntentOp::Field(field) => {
            let Some(node) = db.with_conn(|c| nodes::get_node(c, &item.entity_id))? else {
                return Ok(None);
            };
            let mut m = base("update");
            m.field = Some(field.clone());
            m.value = Some(node_field_value(&node, field));
            Ok(Some(m))
        }
    }
}

/// Maps a feed field name to the node's current column value (JSON).
fn node_field_value(node: &NodeRow, field: &str) -> serde_json::Value {
    use serde_json::json;
    match field {
        "title" => json!(node.title),
        "parentId" => json!(node.parent_id),
        "position" => json!(node.position),
        "type" => json!(node.artifact_type),
        "content" => json!(node.content),
        "sourceUrl" => json!(node.source_url),
        "summary" => json!(node.summary),
        "metadata" => node
            .metadata_json
            .as_ref()
            .and_then(|s| serde_json::from_str(s).ok())
            .unwrap_or(serde_json::Value::Null),
        _ => serde_json::Value::Null,
    }
}

fn resolve_intent(conn: &rusqlite::Connection, item: &PendingIntent) -> DbResult<()> {
    match &item.op {
        IntentOp::Field(field) => {
            intent::resolve_field(conn, &item.entity_kind, &item.entity_id, field, item.version)?;
        }
        _ => {
            intent::resolve_existence(conn, &item.entity_kind, &item.entity_id, item.version)?;
        }
    }
    Ok(())
}

fn drop_intent(conn: &rusqlite::Connection, item: &PendingIntent) -> DbResult<()> {
    match &item.op {
        IntentOp::Field(field) => intent::drop_field(conn, &item.entity_kind, &item.entity_id, field),
        _ => intent::drop_existence(conn, &item.entity_kind, &item.entity_id),
    }
}

fn config_ready(config: &SyncConfig) -> bool {
    !config.api_base_url.trim().is_empty() && config.token.as_deref().unwrap_or("").trim() != ""
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::sync::{PushResult, SyncPullResult};
    use crate::api::ApiResult;
    use std::path::PathBuf;
    use std::sync::Mutex;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn test_db(name: &str) -> Db {
        let nanos = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos();
        let path = std::env::temp_dir().join(format!(
            "aladin_pushsender_test_{name}_{}_{}.sqlite",
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

    fn node(id: &str, kind: &str, title: &str) -> NodeRow {
        NodeRow {
            id: id.to_string(),
            kind: kind.to_string(),
            parent_id: None,
            position: 0,
            title: Some(title.to_string()),
            artifact_type: if kind == "artifact" { Some("note".into()) } else { None },
            content: None,
            source_url: None,
            summary: None,
            metadata_json: None,
            updated_at: 1,
        }
    }

    /// Records every mutation pushed; returns "applied" for each (or "error"
    /// when `reject` is set).
    struct RecordingApi {
        pushed: Mutex<Vec<PushMutation>>,
        reject: bool,
    }

    impl RecordingApi {
        fn new(reject: bool) -> Self {
            Self { pushed: Mutex::new(Vec::new()), reject }
        }
    }

    impl SyncApiClient for RecordingApi {
        fn pull(&self, _config: &SyncConfig, since: i64) -> ApiResult<SyncPullResult> {
            Ok(SyncPullResult { changes: vec![], cursor: since })
        }
        fn push(&self, _config: &SyncConfig, mutations: &[PushMutation]) -> ApiResult<Vec<PushResult>> {
            let mut log = self.pushed.lock().unwrap();
            let mut results = Vec::new();
            for m in mutations {
                log.push(m.clone());
                results.push(PushResult {
                    mutation_id: m.mutation_id.clone(),
                    status: if self.reject { "error".into() } else { "applied".into() },
                    error: None,
                });
            }
            Ok(results)
        }
    }

    #[test]
    fn drains_create_then_field_and_clears_intents() {
        let db = test_db("drain_clear");
        let events = DataEventHub::default();
        db.with_conn(|c| {
            nodes::upsert_local(c, &node("f1", "folder", "Docs"))?;
            intent::enqueue_create(c, "folder", "f1")?;
            // Rename before the create was sent → also a field intent.
            nodes::upsert_local(c, &node("f1", "folder", "Renamed"))?;
            intent::enqueue_field(c, "folder", "f1", "title")?;
            Ok(())
        })
        .unwrap();

        let api = RecordingApi::new(false);
        let stats = push_pending(&db, &events, &config(), &api).unwrap();
        assert_eq!(stats.sent, 2);

        // All intents cleared.
        assert!(db.with_conn(intent::list_pending).unwrap().is_empty());

        // Create sent first (with the latest title from nodes), then the field.
        let pushed = api.pushed.lock().unwrap();
        assert_eq!(pushed.len(), 2);
        assert_eq!(pushed[0].op, "create");
        assert_eq!(pushed[0].title.as_deref(), Some("Renamed"));
        assert_eq!(pushed[1].op, "update");
        assert_eq!(pushed[1].field.as_deref(), Some("title"));
        assert_eq!(pushed[1].value, Some(serde_json::json!("Renamed")));
    }

    #[test]
    fn server_reject_drops_intent_and_reconciles() {
        let db = test_db("reject");
        let events = DataEventHub::default();
        db.with_conn(|c| {
            nodes::upsert_local(c, &node("f1", "folder", "Bad"))?;
            intent::enqueue_create(c, "folder", "f1")?;
            Ok(())
        })
        .unwrap();

        let api = RecordingApi::new(true);
        let stats = push_pending(&db, &events, &config(), &api).unwrap();
        assert_eq!(stats.rejected, 1);
        // Rejected intent is dropped (no infinite retry).
        assert!(db.with_conn(intent::list_pending).unwrap().is_empty());
    }

    #[test]
    fn delete_sends_without_node() {
        let db = test_db("delete_no_node");
        let events = DataEventHub::default();
        db.with_conn(|c| {
            // No node row — a delete intent must still send a delete mutation.
            intent::enqueue_delete(c, "artifact", "gone")?;
            Ok(())
        })
        .unwrap();

        let api = RecordingApi::new(false);
        let stats = push_pending(&db, &events, &config(), &api).unwrap();
        assert_eq!(stats.sent, 1);
        let pushed = api.pushed.lock().unwrap();
        assert_eq!(pushed[0].op, "delete");
        assert_eq!(pushed[0].entity_id, "gone");
    }

    #[test]
    fn unconfigured_session_skips() {
        let db = test_db("push_unconfigured");
        let events = DataEventHub::default();
        db.with_conn(|c| intent::enqueue_create(c, "folder", "f1")).unwrap();
        let blank = SyncConfig { api_base_url: "".into(), token: None };
        let api = RecordingApi::new(false);
        assert_eq!(push_pending(&db, &events, &blank, &api).unwrap(), PushStats::default());
        assert!(api.pushed.lock().unwrap().is_empty());
    }
}
