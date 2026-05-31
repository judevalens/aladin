use serde_json::json;

use crate::{
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

// Data-layer R1-C (client) — the workspace write proxy.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// The Tauri/Rust backend owns the workspace write path: a webview write repo
// invokes a Tauri command, which calls this proxy to the Go REST API and
// returns. The proxy does NOT touch SQLite — the change comes back as a sync
// FRAME (live drain → ws, or pull), so the frame-apply stays the sole cache
// writer, even for the client's own writes. The trait is the test seam.

pub trait WorkspaceWriteApi: Send + Sync {
    /// POST /api/browser/nodes — create a folder or artifact node.
    fn create_node(&self, config: &SyncConfig, body: serde_json::Value) -> ApiResult<()>;
    /// PATCH /api/folders/{id} — rename a folder.
    fn rename_folder(&self, config: &SyncConfig, id: &str, title: &str) -> ApiResult<()>;
    /// PATCH /api/artifacts/{id} — rename (patch) an artifact.
    fn rename_artifact(&self, config: &SyncConfig, id: &str, title: &str) -> ApiResult<()>;
    /// DELETE /api/browser/nodes/{id} — soft-delete a node + its subtree.
    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<()>;
}

#[derive(Default)]
pub struct HttpWorkspaceWriteApi;

impl HttpWorkspaceWriteApi {
    fn base(config: &SyncConfig) -> String {
        config.api_base_url.trim_end_matches('/').to_string()
    }

    fn token(config: &SyncConfig) -> String {
        config.token.clone().unwrap_or_default()
    }
}

impl WorkspaceWriteApi for HttpWorkspaceWriteApi {
    fn create_node(&self, config: &SyncConfig, body: serde_json::Value) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .post(format!("{}/api/browser/nodes", Self::base(config)))
            .bearer_auth(Self::token(config))
            .json(&body)
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }

    fn rename_folder(&self, config: &SyncConfig, id: &str, title: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .patch(format!("{}/api/folders/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .json(&json!({ "title": title }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }

    fn rename_artifact(&self, config: &SyncConfig, id: &str, title: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .patch(format!("{}/api/artifacts/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .json(&json!({ "title": title }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }

    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .delete(format!("{}/api/browser/nodes/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }
}
