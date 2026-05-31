use serde::Deserialize;
use serde_json::json;
use tauri::State;

use crate::api::workspace_write::{HttpWorkspaceWriteApi, WorkspaceWriteApi};
use crate::commands::nodes::{artifact_row_from_node, browser_node_from_node};
use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::browser::BrowserNodeRow;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::{Db, DbError, DbResult};
use crate::events::{DataEvent, DataEventHub};
use crate::sync::{SyncConfig, SyncHandle};

// Data-layer R1-C (client) — the workspace tree write path as Go PROXIES.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// The Tauri/Rust backend owns workspace writes: each command proxies to the Go
// REST API and returns; it does NOT write SQLite. The change comes back as a
// sync FRAME (live drain → ws, or pull), so the frame-apply stays the SOLE cache
// writer, even for the client's own writes. Returns synthesize the legacy shape
// from the input so the calling repo keeps a stable signature; the authoritative
// row arrives via the frame (R1-F drops any optimistic-local assumption).

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

fn require_config(sync: &State<'_, SyncHandle>) -> DbResult<SyncConfig> {
    sync.get().ok_or(DbError::NotInitialized)
}

// --- reads (unchanged: SQLite cache, tombstones filtered in nodes::*) ---

#[tauri::command]
pub fn db_list_browser_nodes(db: State<'_, Db>) -> DbResult<Vec<BrowserNodeRow>> {
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

/// Cache-only upsert of a browser node (NO write proxy — caches a read into the
/// local model, e.g. read-through). Preserves the node's existing fields.
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

// --- writes (proxy to Go; no SQLite, no emit — the frame brings it back) ---

#[tauri::command]
pub fn db_create_browser_node(
    sync: State<'_, SyncHandle>,
    input: LocalBrowserNodeCreateCommand,
) -> DbResult<BrowserCreateResult> {
    let config = require_config(&sync)?;
    let is_artifact = input.kind.eq_ignore_ascii_case("artifact");
    let kind = if is_artifact { "artifact" } else { "folder" }.to_string();

    let mut body = json!({
        "id": input.id,
        "kind": kind,
        "parentId": input.parent_id,
        "title": input.title,
    });
    if is_artifact {
        body["artifact"] = json!({
            "type": input.artifact_type.clone().unwrap_or_else(|| "note".to_string()),
            "content": input.content,
            "summary": input.summary,
            "sourceUrl": input.source_url,
        });
    }

    HttpWorkspaceWriteApi
        .create_node(&config, body)
        .map_err(DbError::Api)?;

    // Synthesize the legacy shape from the input; the authoritative row arrives
    // via the frame (this is NOT written to the cache).
    let synth = NodeRow {
        id: input.id.clone(),
        kind: kind.clone(),
        parent_id: input.parent_id.clone(),
        position: 0,
        title: Some(input.title.clone()),
        artifact_type: if is_artifact {
            input.artifact_type.clone()
        } else {
            None
        },
        content: if is_artifact {
            input.content.clone()
        } else {
            None
        },
        source_url: if is_artifact {
            input.source_url.clone()
        } else {
            None
        },
        summary: if is_artifact {
            input.summary.clone()
        } else {
            None
        },
        metadata_json: None,
        updated_at: input.updated_at,
    };
    let node = browser_node_from_node(synth.clone());
    let artifact = if is_artifact {
        Some(artifact_row_from_node(&synth))
    } else {
        None
    };
    Ok(BrowserCreateResult { node, artifact })
}

#[tauri::command]
pub fn db_rename_browser_node(
    sync: State<'_, SyncHandle>,
    input: LocalBrowserMutationCommand,
) -> DbResult<BrowserNodeRow> {
    let config = require_config(&sync)?;
    // A browser node here is a folder (artifacts rename via db_rename_artifact).
    HttpWorkspaceWriteApi
        .rename_folder(&config, &input.id, &input.title)
        .map_err(DbError::Api)?;

    let synth = NodeRow {
        id: input.id.clone(),
        kind: "folder".to_string(),
        parent_id: input.parent_id.clone(),
        position: 0,
        title: Some(input.title.clone()),
        artifact_type: None,
        content: None,
        source_url: None,
        summary: None,
        metadata_json: None,
        updated_at: input.updated_at,
    };
    Ok(browser_node_from_node(synth))
}

#[tauri::command]
pub fn db_delete_browser_node(
    sync: State<'_, SyncHandle>,
    input: LocalBrowserDeleteCommand,
) -> DbResult<()> {
    let config = require_config(&sync)?;
    HttpWorkspaceWriteApi
        .delete_node(&config, &input.id)
        .map_err(DbError::Api)?;
    Ok(())
}
