use serde::Deserialize;
use serde_json::json;
use tauri::State;

use crate::api::workspace_write::HttpWorkspaceWriteApi;
use crate::commands::nodes::artifact_row_from_node;
use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::repo::workspace_write;
use crate::db::{Db, DbError, DbResult};
use crate::events::{DataEvent, DataEventHub};
use crate::sync::{SyncConfig, SyncHandle};

// Data-layer R1-F (client) — artifact reads + write commands (thin adapters).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Reads serve the legacy ArtifactRow shape from the `nodes` cache (an artifact's
// id IS its node id). Writes are thin adapters over the write REPO
// (db/repo/workspace_write.rs): proxy to Go, then apply the committed result into
// the local cache under the seq guard — so the change appears immediately and the
// later WS frame dedups by seq. Page bodies stay in Yjs/Hocuspocus (separate).

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalArtifactMutationCommand {
    pub id: String,
    pub folder_id: Option<String>,
    pub r#type: Option<String>,
    pub title: String,
    pub content: Option<String>,
    pub summary: Option<String>,
    pub source_url: Option<String>,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalArtifactDeleteCommand {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

fn require_config(sync: &State<'_, SyncHandle>) -> DbResult<SyncConfig> {
    sync.get().ok_or(DbError::NotInitialized)
}

/// Builds the synthesized ArtifactRow returned to the caller (the authoritative
/// row arrives via the frame; this is NOT written to the cache).
fn synth_artifact_row(input: &LocalArtifactMutationCommand) -> ArtifactRow {
    let node = NodeRow {
        id: input.id.clone(),
        kind: "artifact".to_string(),
        parent_id: input.folder_id.clone(),
        position: 0,
        title: Some(input.title.clone()),
        artifact_type: Some(
            input
                .r#type
                .clone()
                .filter(|t| !t.trim().is_empty())
                .unwrap_or_else(|| "note".to_string()),
        ),
        content: input.content.clone(),
        source_url: input.source_url.clone(),
        summary: input.summary.clone(),
        metadata_json: None,
        updated_at: input.updated_at,
    };
    artifact_row_from_node(&node)
}

// --- reads (unchanged: SQLite cache, tombstones filtered in nodes::*) ---

#[tauri::command]
pub fn db_list_artifacts(db: State<'_, Db>) -> DbResult<Vec<ArtifactRow>> {
    db.with_conn(|conn| {
        Ok(nodes::list_nodes(conn)?
            .into_iter()
            .filter(|n| n.kind.eq_ignore_ascii_case("artifact"))
            .map(|n| artifact_row_from_node(&n))
            .collect())
    })
}

#[tauri::command]
pub fn db_get_artifact(db: State<'_, Db>, id: String) -> DbResult<Option<ArtifactRow>> {
    db.with_conn(|conn| {
        Ok(nodes::get_node(conn, &id)?
            .filter(|n| n.kind.eq_ignore_ascii_case("artifact"))
            .map(|n| artifact_row_from_node(&n)))
    })
}

#[tauri::command]
pub fn db_get_artifacts(db: State<'_, Db>, ids: Vec<String>) -> DbResult<Vec<ArtifactRow>> {
    db.with_conn(|conn| {
        let mut out = Vec::new();
        for id in ids {
            if let Some(node) = nodes::get_node(conn, &id)? {
                if node.kind.eq_ignore_ascii_case("artifact") {
                    out.push(artifact_row_from_node(&node));
                }
            }
        }
        Ok(out)
    })
}

/// Cache-only upsert of an artifact (NO write proxy — caches a server/API read,
/// e.g. read-through of an artifact body, into the local model).
#[tauri::command]
pub fn db_upsert_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    row: ArtifactRow,
) -> DbResult<()> {
    let node = db.with_tx(|tx| {
        let existing = nodes::get_node(tx, &row.id)?;
        let position = match &existing {
            Some(n) => n.position,
            None => nodes::next_position(tx, row.folder_id.as_deref())?,
        };
        let node = NodeRow {
            id: row.id.clone(),
            kind: "artifact".to_string(),
            parent_id: row.folder_id.clone(),
            position,
            title: Some(row.title.clone()),
            artifact_type: Some(row.kind.clone()),
            content: row.content.clone(),
            source_url: row.source_url.clone(),
            summary: existing.as_ref().and_then(|n| n.summary.clone()),
            metadata_json: row.metadata_json.clone(),
            updated_at: row.updated_at,
        };
        nodes::upsert_local(tx, &node)?;
        Ok(node)
    })?;
    events.emit(DataEvent::NodeUpserted(node));
    Ok(())
}

// --- writes (thin adapters → write REPO: proxy to Go + seq-guarded cache apply) ---

#[tauri::command]
pub fn db_create_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalArtifactMutationCommand,
) -> DbResult<ArtifactRow> {
    let config = require_config(&sync)?;
    let artifact_type = input
        .r#type
        .clone()
        .filter(|t| !t.trim().is_empty())
        .unwrap_or_else(|| "note".to_string());
    // Create through the unified browser-node endpoint (kind=artifact).
    let body = json!({
        "id": input.id,
        "kind": "artifact",
        "parentId": input.folder_id,
        "title": input.title,
        "artifact": {
            "type": artifact_type,
            "content": input.content,
            "summary": input.summary,
            "sourceUrl": input.source_url,
        },
    });
    let row = workspace_write::create_node(&db, &events, &HttpWorkspaceWriteApi, &config, body)?;
    Ok(artifact_row_from_node(&row))
}

#[tauri::command]
pub fn db_rename_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalArtifactMutationCommand,
) -> DbResult<ArtifactRow> {
    let config = require_config(&sync)?;
    let row =
        workspace_write::rename_artifact(&db, &events, &HttpWorkspaceWriteApi, &config, &input.id, &input.title)?;
    // Fall back to a synthesized row if the artifact isn't cached yet (the WS
    // frame reconciles); keeps the TS repo's return shape stable.
    Ok(row
        .map(|n| artifact_row_from_node(&n))
        .unwrap_or_else(|| synth_artifact_row(&input)))
}

#[tauri::command]
pub fn db_delete_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    sync: State<'_, SyncHandle>,
    input: LocalArtifactDeleteCommand,
) -> DbResult<()> {
    // Delete through the browser-node endpoint so the server tombstones the node
    // spine (and any subtree), not just the artifact row.
    let config = require_config(&sync)?;
    workspace_write::delete_node(&db, &events, &HttpWorkspaceWriteApi, &config, &input.id)
}
