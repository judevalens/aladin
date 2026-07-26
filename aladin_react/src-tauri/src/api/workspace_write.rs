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
    /// PATCH /api/artifacts/{id} — update an artifact's metadata (e.g. typed properties).
    fn update_artifact_metadata(
        &self,
        config: &SyncConfig,
        id: &str,
        metadata: serde_json::Value,
    ) -> ApiResult<WrittenArtifact>;
    /// DELETE /api/browser/nodes/{id} — soft-delete a node + its subtree.
    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<DeleteNodeResult>;
    /// GET /api/artifacts/query — artifacts carrying a typed property (H1c). An empty `value`
    /// matches any value for the key. Returns the raw server JSON array (the caller shapes it).
    fn query_artifacts_by_property(
        &self,
        config: &SyncConfig,
        key: &str,
        value: &str,
    ) -> ApiResult<serde_json::Value>;
    /// GET /api/artifacts/property-facets — the property keys/values in use (for a filter UI).
    fn artifact_property_facets(&self, config: &SyncConfig) -> ApiResult<serde_json::Value>;
    /// GET /api/artifacts/{id}/entities — the entities tagged on an artifact.
    fn list_artifact_entities(&self, config: &SyncConfig, id: &str) -> ApiResult<serde_json::Value>;
    /// POST /api/artifacts/{id}/entities — tag an entity onto an artifact.
    fn attach_artifact_entity(&self, config: &SyncConfig, id: &str, entity_id: &str) -> ApiResult<()>;
    /// DELETE /api/artifacts/{id}/entities/{entityId} — untag it.
    fn detach_artifact_entity(&self, config: &SyncConfig, id: &str, entity_id: &str) -> ApiResult<()>;
    /// PUT /api/artifacts/{id}/entity-mentions — replace the @entity mention set.
    fn sync_artifact_mentions(
        &self,
        config: &SyncConfig,
        id: &str,
        mentions: serde_json::Value,
    ) -> ApiResult<()>;
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

    fn update_artifact_metadata(
        &self,
        config: &SyncConfig,
        id: &str,
        metadata: serde_json::Value,
    ) -> ApiResult<WrittenArtifact> {
        let client = reqwest::blocking::Client::new();
        client
            .patch(format!("{}/api/artifacts/{}", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .json(&json!({ "metadata": metadata }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn query_artifacts_by_property(
        &self,
        config: &SyncConfig,
        key: &str,
        value: &str,
    ) -> ApiResult<serde_json::Value> {
        let client = reqwest::blocking::Client::new();
        let mut req = client
            .get(format!("{}/api/artifacts/query", Self::base(config)))
            .bearer_auth(Self::token(config))
            .query(&[("key", key)]);
        if !value.is_empty() {
            req = req.query(&[("value", value)]);
        }
        req.send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn artifact_property_facets(&self, config: &SyncConfig) -> ApiResult<serde_json::Value> {
        let client = reqwest::blocking::Client::new();
        client
            .get(format!("{}/api/artifacts/property-facets", Self::base(config)))
            .bearer_auth(Self::token(config))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn list_artifact_entities(&self, config: &SyncConfig, id: &str) -> ApiResult<serde_json::Value> {
        let client = reqwest::blocking::Client::new();
        client
            .get(format!("{}/api/artifacts/{}/entities", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?
            .json()
            .map_err(ApiError::from_reqwest)
    }

    fn attach_artifact_entity(&self, config: &SyncConfig, id: &str, entity_id: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .post(format!("{}/api/artifacts/{}/entities", Self::base(config), id))
            .bearer_auth(Self::token(config))
            .json(&json!({ "entityId": entity_id }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }

    fn detach_artifact_entity(&self, config: &SyncConfig, id: &str, entity_id: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .delete(format!(
                "{}/api/artifacts/{}/entities/{}",
                Self::base(config),
                id,
                entity_id
            ))
            .bearer_auth(Self::token(config))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }

    fn sync_artifact_mentions(
        &self,
        config: &SyncConfig,
        id: &str,
        mentions: serde_json::Value,
    ) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .put(format!(
                "{}/api/artifacts/{}/entity-mentions",
                Self::base(config),
                id
            ))
            .bearer_auth(Self::token(config))
            .json(&json!({ "mentions": mentions }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
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
