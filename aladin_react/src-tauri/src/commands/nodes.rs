use tauri::State;

use crate::db::repo::nodes::{self, NodeRow};
use crate::db::{Db, DbResult};
use crate::events::{DataEvent, DataEventHub, EntityDeletedEvent};

// Data-layer redesign, Phase A (client) — reads + local-write mirror for the
// unified `nodes` model. Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// Reads come from `nodes` (authoritative local model, converged by the pull
// engine). Local mutations still flow through the existing browser/artifact
// repos (which push to the server), and additionally mirror their result into
// `nodes` + emit a node event so the nodes-backed UI updates optimistically;
// the server echo reconciles idempotently via pull. The mirror is the local
// half of the eventual write path — only the legacy folders/artifacts writes
// are removed at cutover.

#[tauri::command]
pub fn db_list_nodes(db: State<'_, Db>) -> DbResult<Vec<NodeRow>> {
    db.with_conn(nodes::list_nodes)
}

#[tauri::command]
pub fn db_get_node(db: State<'_, Db>, id: String) -> DbResult<Option<NodeRow>> {
    db.with_conn(|conn| nodes::get_node(conn, &id))
}

/// Mirrors a created/updated node into `nodes` and emits NodeUpserted.
pub(crate) fn mirror_upsert(db: &Db, events: &DataEventHub, row: NodeRow) -> DbResult<()> {
    db.with_conn(|conn| nodes::upsert_local(conn, &row))?;
    events.emit(DataEvent::NodeUpserted(row));
    Ok(())
}

/// Mirrors a local title/parent rename into `nodes` (read-modify-write so no
/// other column is cleared) and emits NodeUpserted.
pub(crate) fn mirror_rename(
    db: &Db,
    events: &DataEventHub,
    id: &str,
    fallback_kind: &str,
    title: String,
    parent_id: Option<String>,
    updated_at: i64,
) -> DbResult<()> {
    let row = db.with_conn(|conn| {
        let mut row = nodes::get_node(conn, id)?.unwrap_or_else(|| blank_node(id, fallback_kind));
        row.title = Some(title.clone());
        row.parent_id = parent_id.clone();
        row.updated_at = updated_at;
        nodes::upsert_local(conn, &row)?;
        Ok(row)
    })?;
    events.emit(DataEvent::NodeUpserted(row));
    Ok(())
}

/// Mirrors a local delete into `nodes` (subtree) and emits NodeDeleted per node.
pub(crate) fn mirror_delete(db: &Db, events: &DataEventHub, id: &str) -> DbResult<()> {
    let removed = db.with_conn(|conn| nodes::delete_subtree(conn, id))?;
    for removed_id in removed {
        events.emit(DataEvent::NodeDeleted(EntityDeletedEvent { id: removed_id }));
    }
    Ok(())
}

fn blank_node(id: &str, kind: &str) -> NodeRow {
    NodeRow {
        id: id.to_string(),
        kind: kind.to_string(),
        parent_id: None,
        position: 0,
        title: None,
        artifact_type: None,
        content: None,
        source_url: None,
        summary: None,
        metadata_json: None,
        updated_at: 0,
    }
}
