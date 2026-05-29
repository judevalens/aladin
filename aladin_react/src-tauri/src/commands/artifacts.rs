use serde::Deserialize;
use tauri::State;

use crate::commands::nodes::artifact_row_from_node;
use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::intent;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::{Db, DbResult};
use crate::events::{DataEvent, DataEventHub, EntityDeletedEvent};

// Data-layer redesign, Phase B (client) — artifact reads + write path over the
// unified `nodes` model. Reads serve the legacy ArtifactRow shape from artifact
// nodes; writes mirror into `nodes` + enqueue an intent (the background sender
// pushes to /api/sync/push) + emit a node event. An artifact's id IS its node
// id. Page bodies stay in page_content/Yjs (separate channel).

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

fn artifact_nodes(db: &Db) -> DbResult<Vec<ArtifactRow>> {
    db.with_conn(|conn| {
        Ok(nodes::list_nodes(conn)?
            .into_iter()
            .filter(|n| n.kind.eq_ignore_ascii_case("artifact"))
            .map(|n| artifact_row_from_node(&n))
            .collect())
    })
}

#[tauri::command]
pub fn db_list_artifacts(db: State<'_, Db>) -> DbResult<Vec<ArtifactRow>> {
    artifact_nodes(&db)
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

/// Local cache upsert of an artifact (no intent — used to cache a server/API
/// read into the local model). Preserves the node's existing placement.
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

#[tauri::command]
pub fn db_create_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    input: LocalArtifactMutationCommand,
) -> DbResult<ArtifactRow> {
    let node = db.with_tx(|tx| {
        let position = nodes::next_position(tx, input.folder_id.as_deref())?;
        let node = NodeRow {
            id: input.id.clone(),
            kind: "artifact".to_string(),
            parent_id: input.folder_id.clone(),
            position,
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
        nodes::upsert_local(tx, &node)?;
        intent::enqueue_create(tx, "artifact", &input.id)?;
        Ok(node)
    })?;
    events.emit(DataEvent::NodeUpserted(node.clone()));
    Ok(artifact_row_from_node(&node))
}

#[tauri::command]
pub fn db_rename_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    input: LocalArtifactMutationCommand,
) -> DbResult<ArtifactRow> {
    let node = db.with_tx(|tx| {
        let mut node = nodes::get_node(tx, &input.id)?.unwrap_or_else(|| NodeRow {
            id: input.id.clone(),
            kind: "artifact".to_string(),
            parent_id: input.folder_id.clone(),
            position: 0,
            title: None,
            artifact_type: Some("note".to_string()),
            content: None,
            source_url: None,
            summary: None,
            metadata_json: None,
            updated_at: input.updated_at,
        });
        node.title = Some(input.title.clone());
        node.updated_at = input.updated_at;
        intent::enqueue_field(tx, "artifact", &input.id, "title")?;
        // Optional fields set only when provided by the caller.
        if let Some(content) = input.content.clone() {
            node.content = Some(content);
            intent::enqueue_field(tx, "artifact", &input.id, "content")?;
        }
        if let Some(summary) = input.summary.clone() {
            node.summary = Some(summary);
            intent::enqueue_field(tx, "artifact", &input.id, "summary")?;
        }
        if let Some(source_url) = input.source_url.clone() {
            node.source_url = Some(source_url);
            intent::enqueue_field(tx, "artifact", &input.id, "sourceUrl")?;
        }
        nodes::upsert_local(tx, &node)?;
        Ok(node)
    })?;
    events.emit(DataEvent::NodeUpserted(node.clone()));
    Ok(artifact_row_from_node(&node))
}

#[tauri::command]
pub fn db_delete_artifact(
    db: State<'_, Db>,
    events: State<'_, DataEventHub>,
    input: LocalArtifactDeleteCommand,
) -> DbResult<()> {
    let removed = db.with_tx(|tx| {
        let removed = nodes::delete_subtree(tx, &input.id)?;
        intent::enqueue_delete(tx, "artifact", &input.id)?;
        Ok(removed)
    })?;
    for id in removed {
        events.emit(DataEvent::NodeDeleted(EntityDeletedEvent { id }));
    }
    Ok(())
}
