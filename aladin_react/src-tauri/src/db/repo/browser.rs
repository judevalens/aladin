use serde::{Deserialize, Serialize};

use crate::db::repo::SyncStatus;

// Data-layer redesign — only the BrowserNodeRow IPC DTO remains. The old
// API-backed BrowserRepo, SqliteBrowserDao, and BrowserEventSubscriber (the
// race-prone tree engine + its outbox/realtime apply) were retired at cutover.
// The tree is now the unified `nodes` model (db::repo::nodes); the command layer
// synthesizes this legacy shape for the frontend (commands::nodes).

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeRow {
    pub id: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub kind: String,
    pub artifact_id: Option<String>,
    pub updated_at: i64,
    pub sync_status: SyncStatus,
    pub version: i64,
}
