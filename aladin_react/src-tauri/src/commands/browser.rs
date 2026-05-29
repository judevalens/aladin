use serde::Deserialize;
use tauri::State;

use crate::commands::nodes::{artifact_row_from_node, browser_node_from_node};
use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::browser::BrowserNodeRow;
use crate::db::repo::intent;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::{Db, DbResult};
use crate::events::{DataEvent, DataEventHub, EntityDeletedEvent};

// Data-layer redesign, Phase B (client) — the workspace tree write path. Each
// mutation writes the unified `nodes` model optimistically, enqueues an intent
// (the background sender pushes it to POST /api/sync/push), and emits a node
// event so the UI updates immediately. The server echo reconciles via pull. The
// client-chosen id IS the server id (the generic write path accepts it), so
// there is no temp-id reassignment. Returns keep the legacy shapes the frontend
// repos expect (synthesized from the input).

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalBrowserMutationCommand {
    pub id: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalBrowserDeleteCommand {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalBrowserNodeCreateCommand {
    pub id: String,
    pub parent_id: Option<String>,
    pub kind: String,
    pub title: String,
    pub artifact_type: Option<String>,
    pub content: Option<String>,
    pub summary: Option<String>,
    pub source_url: Option<String>,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserCreateResult {
    pub node: BrowserNodeRow,
    pub artifact: Option<ArtifactRow>,
}

#[tauri::command]
pub fn db_list_browser_nodes(db: State<'_, Db>) -> DbResult<Vec<BrowserNodeRow>> {
    // Legacy shape, served from the unified nodes model (the new read path is
    // db_list_nodes; this keeps any remaining caller working until cutover).
    db.with_conn(|conn| {
        Ok(nodes::list_nodes(conn)?
            .into_iter()
            .map(browser_node_from_node)
            .collect())
    })
}

#[tauri::command]
pub fn db_get_browser_node(db: State<'_, Db>, id: String) -> DbResult<Option<BrowserNodeRow>> {
    db.with_conn(|conn| Ok(nodes::get_node(conn, &id)?.map(browser_node_from_node)))
}

/// Local cache upsert of a browser node (no intent). Preserves the node's
/// existing artifact fields + placement.
#[tauri::command]
pub fn db_upsert_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    row: BrowserNodeRow,
) -> DbResult<()> {
    let node = db.with_tx(|tx| {
        let existing = nodes::get_node(tx, &row.id)?;
        let is_artifact = row.kind.eq_ignore_ascii_case("artifact");
        let node = NodeRow {
            id: row.id.clone(),
            kind: if is_artifact { "artifact" } else { "folder" }.to_string(),
            parent_id: row.parent_id.clone(),
            position: existing.as_ref().map(|n| n.position).unwrap_or(0),
            title: Some(row.title.clone()),
            artifact_type: existing.as_ref().and_then(|n| n.artifact_type.clone()),
            content: existing.as_ref().and_then(|n| n.content.clone()),
            source_url: existing.as_ref().and_then(|n| n.source_url.clone()),
            summary: existing.as_ref().and_then(|n| n.summary.clone()),
            metadata_json: existing.as_ref().and_then(|n| n.metadata_json.clone()),
            updated_at: row.updated_at,
        };
        nodes::upsert_local(tx, &node)?;
        Ok(node)
    })?;
    events.emit(DataEvent::NodeUpserted(node));
    Ok(())
}

#[tauri::command]
pub fn db_create_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    input: LocalBrowserNodeCreateCommand,
) -> DbResult<BrowserCreateResult> {
    let is_artifact = input.kind.eq_ignore_ascii_case("artifact");
    let kind = if is_artifact { "artifact" } else { "folder" }.to_string();

    let node_row = db.with_tx(|tx| {
        let position = nodes::next_position(tx, input.parent_id.as_deref())?;
        let row = NodeRow {
            id: input.id.clone(),
            kind: kind.clone(),
            parent_id: input.parent_id.clone(),
            position,
            title: Some(input.title.clone()),
            artifact_type: if is_artifact { input.artifact_type.clone() } else { None },
            content: if is_artifact { input.content.clone() } else { None },
            source_url: if is_artifact { input.source_url.clone() } else { None },
            summary: if is_artifact { input.summary.clone() } else { None },
            metadata_json: None,
            updated_at: input.updated_at,
        };
        nodes::upsert_local(tx, &row)?;
        intent::enqueue_create(tx, &kind, &input.id)?;
        Ok(row)
    })?;

    events.emit(DataEvent::NodeUpserted(node_row.clone()));

    let node = browser_node_from_node(node_row.clone());
    let artifact = if is_artifact {
        Some(artifact_row_from_node(&node_row))
    } else {
        None
    };
    Ok(BrowserCreateResult { node, artifact })
}

#[tauri::command]
pub fn db_rename_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    input: LocalBrowserMutationCommand,
) -> DbResult<BrowserNodeRow> {
    let row = db.with_tx(|tx| {
        let mut row = nodes::get_node(tx, &input.id)?.unwrap_or_else(|| NodeRow {
            id: input.id.clone(),
            kind: "folder".to_string(),
            parent_id: input.parent_id.clone(),
            position: 0,
            title: None,
            artifact_type: None,
            content: None,
            source_url: None,
            summary: None,
            metadata_json: None,
            updated_at: input.updated_at,
        });
        row.title = Some(input.title.clone());
        row.updated_at = input.updated_at;
        nodes::upsert_local(tx, &row)?;
        intent::enqueue_field(tx, &row.kind, &input.id, "title")?;
        Ok(row)
    })?;

    events.emit(DataEvent::NodeUpserted(row.clone()));
    Ok(browser_node_from_node(row))
}

#[tauri::command]
pub fn db_delete_browser_node(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    input: LocalBrowserDeleteCommand,
) -> DbResult<()> {
    let removed = db.with_tx(|tx| {
        let kind = nodes::get_node(tx, &input.id)?
            .map(|n| n.kind)
            .unwrap_or_else(|| "folder".to_string());
        let removed = nodes::delete_subtree(tx, &input.id)?;
        intent::enqueue_delete(tx, &kind, &input.id)?;
        Ok(removed)
    })?;

    for id in removed {
        events.emit(DataEvent::NodeDeleted(EntityDeletedEvent { id }));
    }
    Ok(())
}
