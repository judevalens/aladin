use serde_json::json;

use crate::api::workspace_write::WorkspaceWriteApi;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::{Db, DbError, DbResult};
use crate::events::DataEventHub;
use crate::sync::SyncConfig;

// Data-layer R1-F (client) — the workspace write REPO.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// The Tauri/Rust backend owns the workspace write path. This repo does the real
// work behind the thin commands: PROXY the write to Go (via WorkspaceWriteApi),
// then on a successful response APPLY the committed result into the local cache
// via nodes::apply — the SAME guarded repo primitive the WS path uses (no engine
// dependency from a repo) — emit the event it returns, and return the resulting
// row. So the writer's own change shows
// immediately (no wait for the live frame), and the later frame — carrying the
// same seq — is a no-op via the guard. The Frame stays strictly the WS model; it
// never appears here (the REST response is a plain resource + its version).

/// Applies one written entity under the seq guard, emits the derived UI event,
/// and returns the resulting cached row (None for a delete / stale-skip).
fn apply_and_read(
    db: &Db,
    events: &DataEventHub,
    kind: &str,
    id: &str,
    seq: i64,
    op: &str,
    data: Option<serde_json::Value>,
) -> DbResult<Option<NodeRow>> {
    let (row, event) = db.with_tx(|tx| {
        // Apply through the nodes repo's own guarded apply — same primitive the WS
        // path uses, so the local write and the later frame converge by seq, and
        // the repo (not the engine) owns the apply + the dispatch decision.
        let event = nodes::apply(tx, kind, id, seq, op, data.as_ref())?;
        let row = nodes::get_node(tx, id)?;
        Ok((row, event))
    })?;
    if let Some(event) = event {
        events.emit(event);
    }
    Ok(row)
}

/// Re-upsert an existing node with a new title + the server's bumped seq. Rename
/// REST responses omit `position`, so we MERGE onto the cached row rather than
/// reconstruct from the response. If the node isn't cached yet (rename before its
/// create frame landed), there's nothing to merge — the WS frame brings the full
/// node, so we skip the local apply.
fn merge_rename(
    db: &Db,
    events: &DataEventHub,
    id: &str,
    title: &str,
    seq: i64,
) -> DbResult<Option<NodeRow>> {
    let existing = db.with_conn(|conn| nodes::get_node(conn, id))?;
    let Some(existing) = existing else {
        return Ok(None);
    };
    let data = json!({
        "kind": existing.kind,
        "parentId": existing.parent_id,
        "position": existing.position,
        "title": title,
        "type": existing.artifact_type,
        "sourceUrl": existing.source_url,
    });
    apply_and_read(db, events, &existing.kind, id, seq, "upsert", Some(data))
}

/// POST a folder/artifact create, then apply the committed node locally.
pub fn create_node<A: WorkspaceWriteApi>(
    db: &Db,
    events: &DataEventHub,
    api: &A,
    config: &SyncConfig,
    body: serde_json::Value,
) -> DbResult<NodeRow> {
    let res = api.create_node(config, body).map_err(DbError::Api)?;
    let artifact_type = res.artifact.as_ref().and_then(|a| a.artifact_type.clone());
    let source_url = res.artifact.as_ref().and_then(|a| a.source_url.clone());
    let data = json!({
        "kind": res.node.kind,
        "parentId": res.node.parent_id,
        "position": res.node.position,
        "title": res.node.title,
        "type": artifact_type,
        "sourceUrl": source_url,
    });
    let row = apply_and_read(
        db,
        events,
        &res.node.kind,
        &res.node.id,
        res.node.seq as i64,
        "upsert",
        Some(data),
    )?;
    // A create always yields a live row (its seq starts at 1 > stored 0).
    row.ok_or(DbError::NotInitialized)
}

/// PATCH a folder rename, then merge the new title locally.
pub fn rename_folder<A: WorkspaceWriteApi>(
    db: &Db,
    events: &DataEventHub,
    api: &A,
    config: &SyncConfig,
    id: &str,
    title: &str,
) -> DbResult<Option<NodeRow>> {
    let res = api.rename_folder(config, id, title).map_err(DbError::Api)?;
    merge_rename(db, events, &res.id, &res.title, res.seq as i64)
}

/// PATCH an artifact rename, then merge the new title locally.
pub fn rename_artifact<A: WorkspaceWriteApi>(
    db: &Db,
    events: &DataEventHub,
    api: &A,
    config: &SyncConfig,
    id: &str,
    title: &str,
) -> DbResult<Option<NodeRow>> {
    let res = api.rename_artifact(config, id, title).map_err(DbError::Api)?;
    merge_rename(db, events, &res.id, &res.title, res.seq as i64)
}

/// DELETE a node, then soft-delete it locally under the seq guard.
pub fn delete_node<A: WorkspaceWriteApi>(
    db: &Db,
    events: &DataEventHub,
    api: &A,
    config: &SyncConfig,
    id: &str,
) -> DbResult<()> {
    let res = api.delete_node(config, id).map_err(DbError::Api)?;
    // kind only matters for the orphan-tombstone-stub case; a present row updates
    // regardless. Read it before applying; default to folder if already gone.
    let kind = db
        .with_conn(|conn| nodes::get_node(conn, id))?
        .map(|n| n.kind)
        .unwrap_or_else(|| "folder".to_string());
    apply_and_read(db, events, &kind, &res.id, res.seq as i64, "delete", None)?;
    Ok(())
}
