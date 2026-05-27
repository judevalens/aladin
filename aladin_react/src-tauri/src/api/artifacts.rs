use serde::{Deserialize, Serialize};

use crate::{
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

pub trait ArtifactApiClient: Send + Sync {
    fn create_artifact(
        &self,
        config: &SyncConfig,
        input: &ArtifactCreateApiInput<'_>,
    ) -> ApiResult<ArtifactCreateApiRecord>;

    fn rename_artifact(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<ArtifactApiRecord>;

    fn delete_artifact(&self, config: &SyncConfig, id: &str) -> ApiResult<()>;
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ArtifactApiRecord {
    pub id: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub folder_id: Option<String>,
    pub title: String,
    pub content: String,
    pub summary: Option<String>,
    pub source_url: Option<String>,
}

pub struct ArtifactCreateApiInput<'a> {
    pub id: &'a str,
    pub artifact_type: &'a str,
    pub folder_id: Option<&'a str>,
    pub title: &'a str,
    pub content: Option<&'a str>,
    pub summary: Option<&'a str>,
    pub source_url: Option<&'a str>,
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
pub struct ArtifactCreateApiRecord {
    pub artifact: ArtifactApiRecord,
    pub node: BrowserNodeApiRecord,
}

#[derive(Default)]
pub struct ArtifactApi;

impl ArtifactApiClient for ArtifactApi {
    fn create_artifact(
        &self,
        config: &SyncConfig,
        input: &ArtifactCreateApiInput<'_>,
    ) -> ApiResult<ArtifactCreateApiRecord> {
        let client = reqwest::blocking::Client::new();
        // Post-M5 the Go side validates pages on `blocks` (a JSON array)
        // and ignores `content`. We always send an empty blocks array for
        // page artifacts; the actual document is shipped separately via
        // the page_content outbox path (PATCH /api/pages/{id}).
        let mut body = serde_json::json!({
            "type": input.artifact_type,
            "id": input.id,
            "folderId": input.folder_id,
            "title": input.title,
            "summary": input.summary,
            "sourceUrl": input.source_url,
        });
        if input.artifact_type == "page" {
            body["blocks"] = serde_json::json!([]);
        } else {
            body["content"] = serde_json::Value::from(input.content);
        }
        let response = client
            .post(format!(
                "{}/api/artifacts/",
                config.api_base_url.trim_end_matches('/')
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .json(&body)
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }

    fn rename_artifact(
        &self,
        config: &SyncConfig,
        id: &str,
        title: &str,
    ) -> ApiResult<ArtifactApiRecord> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .patch(format!(
                "{}/api/artifacts/{}",
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

    fn delete_artifact(&self, config: &SyncConfig, id: &str) -> ApiResult<()> {
        let client = reqwest::blocking::Client::new();
        client
            .delete(format!(
                "{}/api/artifacts/{}",
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
