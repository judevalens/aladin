use std::collections::HashMap;
use std::sync::Arc;

use rusqlite::Connection;

use crate::api::sync::{Frame, FrameEntity};
use crate::db::repo::{nodes, signals};
use crate::db::DbResult;
use crate::events::DataEvent;

// Data-layer R1-C (client) — the live/pull FRAME dispatcher.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md (§5).
//
// Routes each WS/pull frame entity by `entityKind` to a registered EntityHandler
// for that kind. A handler applies the entity to the kind's repo and returns the
// UI event the repo chose to dispatch (None = nothing to surface). The engine
// owns ONLY dispatch: the seq guard, the cache write, and the event decision all
// live in the repo (nodes::apply). The engine never reaches past a handler into a
// repo's internals, and a repo never depends on the engine. The tree kind (folder
// + artifact → `nodes`) is the only kind today.

/// Handles one live/pull frame entity: apply it to the kind's repo and return the
/// event to dispatch (None = applied-but-not-surfaced, or skipped by the guard).
pub trait EntityHandler: Send + Sync {
    fn apply(&self, conn: &Connection, entity: &FrameEntity) -> DbResult<Option<DataEvent>>;
}

/// The tree handler: folder + artifact entities both live in `nodes`.
struct TreeHandler;

impl EntityHandler for TreeHandler {
    fn apply(&self, conn: &Connection, entity: &FrameEntity) -> DbResult<Option<DataEvent>> {
        nodes::apply(
            conn,
            &entity.entity_kind,
            &entity.entity_id,
            entity.seq as i64,
            &entity.op,
            entity.data.as_ref(),
        )
    }
}

/// The signal handler: claim entities ("signal" kind) live in `signals`.
struct SignalHandler;

impl EntityHandler for SignalHandler {
    fn apply(&self, conn: &Connection, entity: &FrameEntity) -> DbResult<Option<DataEvent>> {
        signals::apply(
            conn,
            &entity.entity_id,
            entity.seq as i64,
            &entity.op,
            entity.data.as_ref(),
        )
    }
}

/// Routes frame entities to handlers by entity kind.
pub struct Registry {
    handlers: HashMap<String, Arc<dyn EntityHandler>>,
}

impl Registry {
    /// The registry: the tree kind (folder + artifact → `nodes`) and the signal
    /// kind (claims → `signals`).
    pub fn tree() -> Self {
        let tree: Arc<dyn EntityHandler> = Arc::new(TreeHandler);
        let mut handlers: HashMap<String, Arc<dyn EntityHandler>> = HashMap::new();
        handlers.insert("folder".to_string(), tree.clone());
        handlers.insert("artifact".to_string(), tree);
        handlers.insert("signal".to_string(), Arc::new(SignalHandler));
        Self { handlers }
    }

    fn get(&self, kind: &str) -> Option<&Arc<dyn EntityHandler>> {
        self.handlers.get(kind)
    }
}

/// Applies one frame's entities to their kinds' repos (in the caller's tx) and
/// returns the events the repos chose to dispatch, in order. An unknown kind is
/// skipped (forward-compat). Does NOT advance the cursor (the pull caller does;
/// live does not) or emit — the caller emits post-commit.
pub fn apply_frame(
    conn: &Connection,
    registry: &Registry,
    frame: &Frame,
) -> DbResult<Vec<DataEvent>> {
    let mut events = Vec::new();
    for entity in &frame.entities {
        let Some(handler) = registry.get(&entity.entity_kind) else {
            continue; // unknown kind: a newer server may emit kinds we don't handle
        };
        if let Some(event) = handler.apply(conn, entity)? {
            events.push(event);
        }
    }
    Ok(events)
}
