use serde::{Deserialize, Serialize};

use crate::db::repo::SyncStatus;

// Data-layer redesign — only the ArtifactRow IPC DTO remains. The old
// API-backed ArtifactRepo + outbox path was retired at cutover; artifacts are
// now rows of the unified `nodes` model (db::repo::nodes), and the command layer
// synthesizes this legacy shape for the frontend (commands::nodes).

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactRow {
    pub id: String,
    pub folder_id: Option<String>,
    pub title: String,
    pub kind: String,
    pub content: Option<String>,
    pub source_url: Option<String>,
    pub resource_url: Option<String>,
    pub metadata_json: Option<String>,
    pub updated_at: i64,
    pub sync_status: SyncStatus,
    pub version: i64,
}
