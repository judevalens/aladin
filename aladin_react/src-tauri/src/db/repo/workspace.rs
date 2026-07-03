use std::sync::Arc;

use serde::{Deserialize, Serialize};
use serde_json::json;

use crate::api::workspace_write::WorkspaceWriteApi;
use crate::db::repo::artifacts::ArtifactRow;
use crate::db::repo::browser::BrowserNodeRow;
use crate::db::repo::nodes::{self, NodeRow};
use crate::db::repo::SyncStatus;
use crate::db::{Db, DbError, DbResult};
use crate::events::{DataEvent, DataEventHub};
use crate::sync::{SyncConfig, SyncHandle};

// Data-layer (client) — the workspace REPO.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// One stateful repo for the workspace tree (folders + artifacts both live in the
// `nodes` cache). It HOLDS its logical deps — the event hub, the sync handle (for
// the bearer/config), and the write API (the test seam) — so callers don't thread
// them. `db` is the ambient resource, passed per call (the repo owns the tx
// boundary for writes; reads run on a borrowed connection). Commands are thin
// adapters that just delegate here.
//
// Reads materialize from `nodes`. Writes PROXY to Go, then apply the committed
// result into the cache via nodes::apply (the same guarded primitive the WS frame
// path uses) and emit the event the repo returns — so the writer's own change
// shows immediately and the later frame dedups by seq. The Frame is the WS model
// and never appears here. Page CONTENT rides Yjs/Hocuspocus (a separate channel).

// --- command input DTOs (the Tauri invoke arg shapes) ---

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserMutationInput {
    pub id: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserDeleteInput {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeCreateInput {
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

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactMutationInput {
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

#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactDeleteInput {
    pub id: String,
    pub updated_at: i64,
    pub mutation_id: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrowserCreateResult {
    pub node: BrowserNodeRow,
    pub artifact: Option<ArtifactRow>,
}

pub struct WorkspaceRepo {
    events: DataEventHub,
    sync: SyncHandle,
    api: Arc<dyn WorkspaceWriteApi>,
}

impl WorkspaceRepo {
    pub fn new(events: DataEventHub, sync: SyncHandle, api: Arc<dyn WorkspaceWriteApi>) -> Self {
        Self { events, sync, api }
    }

    fn config(&self) -> DbResult<SyncConfig> {
        self.sync.get().ok_or(DbError::NotInitialized)
    }

    // --- reads (materialize from the `nodes` cache; tombstones filtered there) ---

    pub fn list_nodes(&self, db: &Db) -> DbResult<Vec<NodeRow>> {
        db.with_conn(nodes::list_nodes)
    }

    pub fn get_node(&self, db: &Db, id: &str) -> DbResult<Option<NodeRow>> {
        db.with_conn(|conn| nodes::get_node(conn, id))
    }

    pub fn list_browser_nodes(&self, db: &Db) -> DbResult<Vec<BrowserNodeRow>> {
        db.with_conn(|conn| {
            Ok(nodes::list_nodes(conn)?
                .into_iter()
                .map(browser_node_from_node)
                .collect())
        })
    }

    pub fn get_browser_node(&self, db: &Db, id: &str) -> DbResult<Option<BrowserNodeRow>> {
        db.with_conn(|conn| Ok(nodes::get_node(conn, id)?.map(browser_node_from_node)))
    }

    pub fn list_artifacts(&self, db: &Db) -> DbResult<Vec<ArtifactRow>> {
        db.with_conn(|conn| {
            Ok(nodes::list_nodes(conn)?
                .into_iter()
                .filter(|n| n.kind.eq_ignore_ascii_case("artifact"))
                .map(|n| artifact_row_from_node(&n))
                .collect())
        })
    }

    pub fn get_artifact(&self, db: &Db, id: &str) -> DbResult<Option<ArtifactRow>> {
        db.with_conn(|conn| {
            Ok(nodes::get_node(conn, id)?
                .filter(|n| n.kind.eq_ignore_ascii_case("artifact"))
                .map(|n| artifact_row_from_node(&n)))
        })
    }

    pub fn get_artifacts(&self, db: &Db, ids: &[String]) -> DbResult<Vec<ArtifactRow>> {
        db.with_conn(|conn| {
            let mut out = Vec::new();
            for id in ids {
                if let Some(node) = nodes::get_node(conn, id)? {
                    if node.kind.eq_ignore_ascii_case("artifact") {
                        out.push(artifact_row_from_node(&node));
                    }
                }
            }
            Ok(out)
        })
    }

    // --- cache-only upserts (NO proxy — cache a server/API read, e.g. read-through) ---

    pub fn upsert_browser_node(&self, db: &Db, row: BrowserNodeRow) -> DbResult<()> {
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
        self.events.emit(DataEvent::NodeUpserted(node));
        Ok(())
    }

    pub fn upsert_artifact(&self, db: &Db, row: ArtifactRow) -> DbResult<()> {
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
        self.events.emit(DataEvent::NodeUpserted(node));
        Ok(())
    }

    // --- writes (proxy to Go, then apply the committed result via nodes::apply) ---

    pub fn create_browser_node(
        &self,
        db: &Db,
        input: BrowserNodeCreateInput,
    ) -> DbResult<BrowserCreateResult> {
        let config = self.config()?;
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
        let res = self.api.create_node(&config, body).map_err(DbError::Api)?;
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
        let row = self
            .apply_and_read(
                db,
                &res.node.kind,
                &res.node.id,
                res.node.seq as i64,
                "upsert",
                Some(data),
            )?
            .ok_or(DbError::NotInitialized)?; // a create always yields a live row
        let artifact = if row.kind.eq_ignore_ascii_case("artifact") {
            Some(artifact_row_from_node(&row))
        } else {
            None
        };
        Ok(BrowserCreateResult {
            node: browser_node_from_node(row),
            artifact,
        })
    }

    pub fn rename_browser_node(
        &self,
        db: &Db,
        input: BrowserMutationInput,
    ) -> DbResult<BrowserNodeRow> {
        let config = self.config()?;
        // A browser node here is a folder (artifacts rename via rename_artifact).
        let res = self
            .api
            .rename_folder(&config, &input.id, &input.title)
            .map_err(DbError::Api)?;
        let node = self
            .merge_rename(db, &res.id, &res.title, res.seq as i64)?
            .unwrap_or_else(|| NodeRow {
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
            });
        Ok(browser_node_from_node(node))
    }

    pub fn delete_browser_node(&self, db: &Db, input: BrowserDeleteInput) -> DbResult<()> {
        let config = self.config()?;
        let res = self
            .api
            .delete_node(&config, &input.id)
            .map_err(DbError::Api)?;
        self.apply_delete(db, &res.id, res.seq as i64)
    }

    pub fn create_artifact(&self, db: &Db, input: ArtifactMutationInput) -> DbResult<ArtifactRow> {
        let config = self.config()?;
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
        let res = self.api.create_node(&config, body).map_err(DbError::Api)?;
        let a_type = res.artifact.as_ref().and_then(|a| a.artifact_type.clone());
        let source_url = res.artifact.as_ref().and_then(|a| a.source_url.clone());
        let data = json!({
            "kind": res.node.kind,
            "parentId": res.node.parent_id,
            "position": res.node.position,
            "title": res.node.title,
            "type": a_type,
            "sourceUrl": source_url,
        });
        let row = self
            .apply_and_read(
                db,
                &res.node.kind,
                &res.node.id,
                res.node.seq as i64,
                "upsert",
                Some(data),
            )?
            .ok_or(DbError::NotInitialized)?;
        Ok(artifact_row_from_node(&row))
    }

    pub fn rename_artifact(&self, db: &Db, input: ArtifactMutationInput) -> DbResult<ArtifactRow> {
        let config = self.config()?;
        let res = self
            .api
            .rename_artifact(&config, &input.id, &input.title)
            .map_err(DbError::Api)?;
        Ok(self
            .merge_rename(db, &res.id, &res.title, res.seq as i64)?
            .map(|n| artifact_row_from_node(&n))
            .unwrap_or_else(|| synth_artifact_row(&input)))
    }

    pub fn delete_artifact(&self, db: &Db, input: ArtifactDeleteInput) -> DbResult<()> {
        let config = self.config()?;
        // Delete through the browser-node endpoint so the server tombstones the
        // node spine (and any subtree), not just the artifact row.
        let res = self
            .api
            .delete_node(&config, &input.id)
            .map_err(DbError::Api)?;
        self.apply_delete(db, &res.id, res.seq as i64)
    }

    // --- internal: apply the committed write result under the seq guard + emit ---

    /// Apply via nodes::apply (the shared guarded primitive), emit the event the
    /// repo returns, and read back the resulting row.
    fn apply_and_read(
        &self,
        db: &Db,
        kind: &str,
        id: &str,
        seq: i64,
        op: &str,
        data: Option<serde_json::Value>,
    ) -> DbResult<Option<NodeRow>> {
        let (row, event) = db.with_tx(|tx| {
            let event = nodes::apply(tx, kind, id, seq, op, data.as_ref())?;
            Ok((nodes::get_node(tx, id)?, event))
        })?;
        if let Some(event) = event {
            self.events.emit(event);
        }
        Ok(row)
    }

    /// Re-upsert an existing node with a new title + the server's bumped seq. The
    /// rename REST response omits `position`, so MERGE onto the cached row rather
    /// than reconstruct. If the node isn't cached yet, skip — the WS frame brings
    /// the full node.
    fn merge_rename(&self, db: &Db, id: &str, title: &str, seq: i64) -> DbResult<Option<NodeRow>> {
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
        self.apply_and_read(db, &existing.kind, id, seq, "upsert", Some(data))
    }

    fn apply_delete(&self, db: &Db, id: &str, seq: i64) -> DbResult<()> {
        // kind only matters for the orphan-tombstone-stub case; a present row
        // updates regardless. Read it before applying; default to folder if gone.
        let kind = db
            .with_conn(|conn| nodes::get_node(conn, id))?
            .map(|n| n.kind)
            .unwrap_or_else(|| "folder".to_string());
        self.apply_and_read(db, &kind, id, seq, "delete", None)?;
        Ok(())
    }
}

/// Builds the synthesized ArtifactRow returned when a rename target isn't cached
/// yet (the WS frame reconciles); keeps the TS repo's return shape stable.
fn synth_artifact_row(input: &ArtifactMutationInput) -> ArtifactRow {
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
/// nodes.content (notes/links); pages keep their body in Yjs, so it's empty there.
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
