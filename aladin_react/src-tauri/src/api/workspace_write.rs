use serde::Deserialize;
use serde_json::json;

use crate::{
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

// Data-layer R1-C/R1-F (client) — the workspace write transport.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Pure HTTP transport: the Tauri/Rust write-repo (db/repo/workspace_write.rs)
// calls these to PROXY a workspace write to the Go REST API, and gets back the
// committed resource representation + its `seq` (the version). The repo then
// applies that result into the local cache under the SAME per-entity seq guard
// the WS frame uses, so the row appears immediately and the later live frame is
// a no-op. The Frame is strictly the WS model and never appears here. The trait
// is the test seam.

/// The committed light node a write returns (mirrors Go service.BrowserNodeResponse).
/// `seq` is the entity version (decimal string on the wire). `position` is present
/// for create; rename DTOs omit it (the repo merges onto the existing cache row).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WrittenNode {
    pub id: String,
    #[serde(default)]
    pub parent_id: Option<String>,
    pub kind: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub position: i64,
    #[serde(with = "crate::api::sync::string_u64")]
    pub seq: u64,
}

/// The artifact body a create/rename returns (mirrors Go service.ArtifactResponse).
/// Carries the light artifact fields (type/sourceUrl) the node row needs, plus seq.
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WrittenArtifact {
    pub id: String,
    #[serde(default, rename = "type")]
    pub artifact_type: Option<String>,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub source_url: Option<String>,
    #[serde(default, with = "crate::api::sync::string_u64")]
    pub seq: u64,
}

/// POST /api/browser/nodes response (mirrors Go service.BrowserNodeCreateResponse).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateNodeResult {
    pub node: WrittenNode,
    #[serde(default)]
    pub artifact: Option<WrittenArtifact>,
}

/// PATCH /api/folders/{id} response (mirrors Go service.FolderNode).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RenameFolderResult {
    pub id: String,
    #[serde(default)]
    pub title: String,
    #[serde(with = "crate::api::sync::string_u64")]
    pub seq: u64,
}

/// DELETE /api/browser/nodes/{id} response (mirrors Go service.NodeDeleteResult).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeleteNodeResult {
    pub id: String,
    #[serde(with = "crate::api::sync::string_u64")]
    pub seq: u64,
}

pub trait WorkspaceWriteApi: Send + Sync {
    /// POST /api/browser/nodes — create a folder or artifact node.
    fn create_node(
        &self,
        config: &SyncConfig,
        body: serde_json::Value,
    ) -> ApiResult<CreateNodeResult>;
    /// PATCH /api/folders/{id} — rename a folder.
    fn rename_folder(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<RenameFolderResult>;
    /// PATCH /api/artifacts/{id} — rename (patch) an artifact.
    fn rename_artifact(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<WrittenArtifact>;
    /// DELETE /api/browser/nodes/{id} — soft-delete a node + its subtree.
    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<DeleteNodeResult>;
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
    fn create_node(
        &self,
        config: &SyncConfig,
        body: serde_json::Value,
    ) -> ApiResult<CreateNodeResult> {
        let client = reqwest::blocking::Client::new();
        client
            .post(format!("{}/api/browser/nodes", Self::base(config)))
            .bearer_auth(Self::token(config))
            .json(&body)
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn rename_folder(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<RenameFolderResult> {
        let client = reqwest::blocking::Client::new();
        client
            .patch(format!("{}/api/folders/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .json(&json!({ "title": title }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn rename_artifact(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<WrittenArtifact> {
        let client = reqwest::blocking::Client::new();
        client
            .patch(format!("{}/api/artifacts/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .json(&json!({ "title": title }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<DeleteNodeResult> {
        let client = reqwest::blocking::Client::new();
        client
            .delete(format!("{}/api/browser/nodes/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }
}
