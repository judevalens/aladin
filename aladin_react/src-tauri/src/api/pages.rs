use serde::{Deserialize, Serialize};

use crate::{
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

/// Single-page Go API surface. The Rust sync engine uses this to push local
/// `page_content` mutations to Postgres via `PATCH /api/pages/{id}` and
/// (for the cold-cache pull path) read back via `GET /api/pages/{id}`.
pub trait PageApiClient: Send + Sync {
    fn get_page(&self, config: &SyncConfig, id: &str) -> ApiResult<PageApiRecord>;
    fn save_page(
        &self,
        config: &SyncConfig,
        id: &str,
        input: &PageSaveApiInput<'_>,
    ) -> ApiResult<PageApiRecord>;
}

#[derive(Debug, Deserialize, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct PageApiRecord {
    pub id: String,
    pub title: String,
    /// Raw blocks JSON. Round-tripped opaquely; this layer doesn't parse
    /// the BlockNote schema.
    pub blocks: serde_json::Value,
    pub revision: i64,
    pub updated_at: String,
}

pub struct PageSaveApiInput<'a> {
    pub blocks: &'a serde_json::Value,
    pub revision: i64,
}

#[derive(Default)]
pub struct PageApi;

impl PageApiClient for PageApi {
    fn get_page(&self, config: &SyncConfig, id: &str) -> ApiResult<PageApiRecord> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .get(format!(
                "{}/api/pages/{}",
                config.api_base_url.trim_end_matches('/'),
                id
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }

    fn save_page(
        &self,
        config: &SyncConfig,
        id: &str,
        input: &PageSaveApiInput<'_>,
    ) -> ApiResult<PageApiRecord> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .patch(format!(
                "{}/api/pages/{}",
                config.api_base_url.trim_end_matches('/'),
                id
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .json(&serde_json::json!({
                "blocks": input.blocks,
                "revision": input.revision,
            }))
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }
}
