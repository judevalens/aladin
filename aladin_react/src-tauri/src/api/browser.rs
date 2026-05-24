use serde::{Deserialize, Serialize};

use crate::{
    api::artifacts::ArtifactApiRecord,
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

pub trait BrowserApiClient: Send + Sync {
    fn get_tree(&self, config: &SyncConfig) -> ApiResult<Vec<BrowserTreeApiRecord>>;

    fn create_node(
        &self,
        config: &SyncConfig,
        input: &BrowserNodeCreateApiInput,
    ) -> ApiResult<BrowserNodeCreateApiRecord>;

    fn rename_folder(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<BrowserNodeApiRecord>;

    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<()>;
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeApiRecord {
    pub id: String,
    pub parent_id: Option<String>,
    pub kind: String,
    pub title: String,
    pub artifact_id: Option<String>,
    pub position: i64,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserArtifactCreateApiPayload {
    pub id: Option<String>,
    #[serde(rename = "type")]
    pub artifact_type: String,
    pub content: Option<String>,
    pub summary: Option<String>,
    pub source_url: Option<String>,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeCreateApiInput {
    pub id: String,
    pub kind: String,
    pub parent_id: Option<String>,
    pub title: String,
    pub artifact: Option<BrowserArtifactCreateApiPayload>,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserNodeCreateApiRecord {
    pub node: BrowserNodeApiRecord,
    pub artifact: Option<ArtifactApiRecord>,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct BrowserTreeApiRecord {
    pub id: String,
    pub parent_id: Option<String>,
    pub kind: String,
    pub title: String,
    pub artifact_id: Option<String>,
    pub artifact_type: Option<String>,
    pub updated_at: Option<String>,
    pub children: Vec<BrowserTreeApiRecord>,
}

#[derive(Default)]
pub struct BrowserApi;

impl BrowserApiClient for BrowserApi {
    fn get_tree(&self, config: &SyncConfig) -> ApiResult<Vec<BrowserTreeApiRecord>> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .get(format!(
                "{}/api/browser/tree",
                config.api_base_url.trim_end_matches('/')
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }

    fn create_node(
        &self,
        config: &SyncConfig,
        input: &BrowserNodeCreateApiInput,
    ) -> ApiResult<BrowserNodeCreateApiRecord> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .post(format!(
                "{}/api/browser/nodes",
                config.api_base_url.trim_end_matches('/')
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .json(input)
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }

    fn rename_folder(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<BrowserNodeApiRecord> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .patch(format!(
                "{}/api/folders/{}",
                config.api_base_url.trim_end_matches('/'),
                id
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .json(&serde_json::json!({ "title": title }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }

    fn delete_node(&self, config: &SyncConfig, id: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .delete(format!(
                "{}/api/browser/nodes/{}",
                config.api_base_url.trim_end_matches('/'),
                id
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        Ok(())
    }
}
