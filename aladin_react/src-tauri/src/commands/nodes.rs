use tauri::State;

use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::browser::BrowserNodeRow;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::repo::SyncStatus;
use crate::db::{Db, DbResult};

// Data-layer redesign, Phase A/B (client) — reads + legacy-shape adapters over
// the unified `nodes` model. Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// `nodes` is the authoritative local model (converged by pull, written by the
// intent write path). These commands read it directly; the adapters synthesize
// the legacy BrowserNodeRow / ArtifactRow shapes the frontend repos still
// expect, so the read flip needs no further frontend churn.

#[tauri::command]
pub fn db_list_nodes(db: State<'_, Db>) -> DbResult<Vec<NodeRow>> {
    db.with_conn(nodes::list_nodes)
}

#[tauri::command]
pub fn db_get_node(db: State<'_, Db>, id: String) -> DbResult<Option<NodeRow>> {
    db.with_conn(|conn| nodes::get_node(conn, &id))
}

/// Legacy BrowserNodeRow view of a unified node (artifact_id == id for artifacts).
pub(crate) fn browser_node_from_node(node: NodeRow) -> BrowserNodeRow {
    let is_artifact = node.kind.eq_ignore_ascii_case("artifact");
    BrowserNodeRow {
        id: node.id.clone(),
        parent_id: node.parent_id,
        title: node.title.unwrap_or_default(),
        kind: node.kind,
        artifact_id: if is_artifact { Some(node.id) } else { None },
        updated_at: node.updated_at,
        sync_status: SyncStatus::Pending,
        version: 0,
    }
}

/// Legacy ArtifactRow view of an artifact node. Note content lives in
/// nodes.content (notes/links); pages keep their body in page_content/Yjs, so
/// nodes.content is empty for pages by design.
pub(crate) fn artifact_row_from_node(node: &NodeRow) -> ArtifactRow {
    ArtifactRow {
        id: node.id.clone(),
        folder_id: node.parent_id.clone(),
        title: node.title.clone().unwrap_or_default(),
        kind: node
            .artifact_type
            .clone()
            .unwrap_or_else(|| "note".to_string()),
        content: node.content.clone(),
        source_url: node.source_url.clone(),
        resource_url: None,
        metadata_json: node
            .summary
            .clone()
            .map(|summary| serde_json::json!({ "summary": summary }).to_string())
            .or_else(|| node.metadata_json.clone()),
        updated_at: node.updated_at,
        sync_status: SyncStatus::Pending,
        version: 0,
    }
}
