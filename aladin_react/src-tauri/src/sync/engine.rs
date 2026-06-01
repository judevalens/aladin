use std::collections::HashMap;
use std::sync::Arc;

use rusqlite::Connection;

use crate::api::sync::Frame;
use crate::db::repo::nodes;
use crate::db::DbResult;
use crate::events::{DataEvent, EntityDeletedEvent};

// Data-layer R1-C (client) — the generic frame apply engine.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md (§5).
//
// The engine is entity-agnostic: it routes each frame entity by `entityKind` to
// a registered EntityHandler, guarded by the per-entity `seq`. The handler owns
// "decode data → my typed table" + soft-delete; the engine owns dispatch + the
// guard. The caller owns the transaction + cursor advance (pull advances it;
// live does not) + event emission. Adding a kind = a table + a handler, ZERO
// engine change. The tree kind (folder + artifact → `nodes`) is the first one.

/// A per-kind apply target. Implemented over a typed cache table.
pub trait EntityHandler: Send + Sync {
    /// Last applied seq for this entity, INCLUDING tombstones (0 if never seen).
    fn stored_seq(&self, conn: &Connection, id: &str) -> DbResult<i64>;
    /// Apply an upsert: write typed columns, set seq, clear the tombstone.
    fn upsert(
        &self,
        conn: &Connection,
        id: &str,
        seq: i64,
        data: Option<&serde_json::Value>,
    ) -> DbResult<()>;
    /// Apply a delete: SOFT delete (tombstone), retaining seq.
    fn soft_delete(&self, conn: &Connection, id: &str, kind: &str, seq: i64) -> DbResult<()>;
}

/// The tree handler: folder + artifact entities both live in `nodes`.
struct TreeHandler;

impl EntityHandler for TreeHandler {
    fn stored_seq(&self, conn: &Connection, id: &str) -> DbResult<i64> {
        nodes::stored_seq(conn, id)
    }
    fn upsert(
        &self,
        conn: &Connection,
        id: &str,
        seq: i64,
        data: Option<&serde_json::Value>,
    ) -> DbResult<()> {
        nodes::apply_upsert(conn, id, seq, data)
    }
    fn soft_delete(&self, conn: &Connection, id: &str, kind: &str, seq: i64) -> DbResult<()> {
        nodes::apply_soft_delete(conn, id, kind, seq)
    }
}

/// Routes frame entities to handlers by entity kind.
pub struct Registry {
    handlers: HashMap<String, Arc<dyn EntityHandler>>,
}

impl Registry {
    /// The R1 registry: the tree kind (folder + artifact → `nodes`).
    pub fn tree() -> Self {
        let tree: Arc<dyn EntityHandler> = Arc::new(TreeHandler);
        let mut handlers: HashMap<String, Arc<dyn EntityHandler>> = HashMap::new();
        handlers.insert("folder".to_string(), tree.clone());
        handlers.insert("artifact".to_string(), tree);
        Self { handlers }
    }

    fn get(&self, kind: &str) -> Option<&Arc<dyn EntityHandler>> {
        self.handlers.get(kind)
    }
}

/// Applies one frame's entities into the cache, in the caller's transaction,
/// under the per-entity seq guard. Returns the touched entity ids (deduped, in
/// first-seen order) so the caller can derive UI events from final state. Does
/// NOT advance the cursor — the pull caller does that in the same txn; live does
/// not. An unknown kind is skipped (forward-compat).
pub fn apply_frame(conn: &Connection, registry: &Registry, frame: &Frame) -> DbResult<Vec<String>> {
    let mut touched: Vec<String> = Vec::new();
    for entity in &frame.entities {
        let applied = apply_one(
            conn,
            registry,
            &entity.entity_kind,
            &entity.entity_id,
            entity.seq as i64,
            &entity.op,
            entity.data.as_ref(),
        )?;
        if applied && !touched.iter().any(|id| id == &entity.entity_id) {
            touched.push(entity.entity_id.clone());
        }
    }
    Ok(touched)
}

/// Applies ONE entity under the per-entity seq guard, outside any Frame. The
/// write path uses this to apply the REST write response directly into the cache
/// (the Frame is strictly the WS model and must not leak into REST), reusing the
/// exact same guard + handlers as the live/pull path. Returns true if applied
/// (false = unknown kind or stale/duplicate seq). An unknown kind is skipped.
pub fn apply_one(
    conn: &Connection,
    registry: &Registry,
    kind: &str,
    id: &str,
    seq: i64,
    op: &str,
    data: Option<&serde_json::Value>,
) -> DbResult<bool> {
    let Some(handler) = registry.get(kind) else {
        return Ok(false);
    };
    if seq <= handler.stored_seq(conn, id)? {
        return Ok(false); // stale / duplicate / out-of-order — the seq guard (§3a)
    }
    match op {
        "delete" => handler.soft_delete(conn, id, kind, seq)?,
        _ => handler.upsert(conn, id, seq, data)?,
    }
    Ok(true)
}

/// Derives one UI event per touched id from its final committed state: a live
/// row → NodeUpserted; a tombstoned/absent row → NodeDeleted.
pub fn derive_events(conn: &Connection, touched: Vec<String>) -> DbResult<Vec<DataEvent>> {
    let mut out = Vec::with_capacity(touched.len());
    for id in touched {
        match nodes::get_node(conn, &id)? {
            Some(row) => out.push(DataEvent::NodeUpserted(row)),
            None => out.push(DataEvent::NodeDeleted(EntityDeletedEvent { id })),
        }
    }
    Ok(out)
}
