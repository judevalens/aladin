use serde::{Deserialize, Serialize};

use crate::{
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

// Data-layer redesign, Phase A (client) — the read/convergence pull client.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// Mirrors the server's workspace change feed (backend_v2/internal/repo/
// sync_postgres.go). GET /api/sync/pull?since=<cursor> returns the coalesced
// latest-per-(entity,field) changes in seq order plus the new cursor. The
// client applies each change through the per-field seq guard into `nodes`.

/// One row of the workspace_changes feed. `field`/`value` are present only for
/// `op == "update"` (field-level); `create`/`delete` are entity-level.
/// `value` is raw JSON: a string for title/parentId/type/..., a number for
/// position, an object for metadata, or null (e.g. parentId at the root).
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncChange {
    pub seq: i64,
    pub entity_kind: String,
    pub entity_id: String,
    pub op: String,
    #[serde(default)]
    pub field: Option<String>,
    #[serde(default)]
    pub value: Option<serde_json::Value>,
    #[serde(default)]
    pub mutation_id: Option<String>,
}

/// Delta response: coalesced changes (seq-ordered) + the new cursor (feed
/// high-water for this user, or the input cursor when nothing is newer).
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncPullResult {
    #[serde(default)]
    pub changes: Vec<SyncChange>,
    pub cursor: i64,
}

/// One client intent pushed to POST /api/sync/push. For op="update" it is a
/// generic setField (field + value); for op="create" it carries the initial
/// fields; for op="delete" only entity identity. Mirrors the Go repo.Mutation.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PushMutation {
    pub mutation_id: String,
    pub op: String,
    pub entity_kind: String,
    pub entity_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub field: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub value: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parent_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub position: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub artifact_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub summary: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_url: Option<String>,
}

/// Per-mutation outcome from the push endpoint. `mutation_id` mirrors the wire
/// response for traceability even though the serial sender keys off position.
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
#[allow(dead_code)]
pub struct PushResult {
    pub mutation_id: String,
    pub status: String, // "applied" | "duplicate" | "error"
    #[serde(default)]
    pub error: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
struct PushResponse {
    #[serde(default)]
    results: Vec<PushResult>,
}

#[derive(Debug, Clone, Serialize)]
struct PushRequest<'a> {
    mutations: &'a [PushMutation],
}

pub trait SyncApiClient: Send + Sync {
    /// Pulls the coalesced change-feed delta with seq > `since`.
    fn pull(&self, config: &SyncConfig, since: i64) -> ApiResult<SyncPullResult>;

    /// Pushes a batch of mutations and returns a per-mutation result.
    fn push(&self, config: &SyncConfig, mutations: &[PushMutation]) -> ApiResult<Vec<PushResult>>;
}

#[derive(Default)]
pub struct SyncApi;

impl SyncApiClient for SyncApi {
    fn pull(&self, config: &SyncConfig, since: i64) -> ApiResult<SyncPullResult> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .get(format!(
                "{}/api/sync/pull",
                config.api_base_url.trim_end_matches('/')
            ))
            .query(&[("since", since.to_string())])
            .bearer_auth(config.token.clone().unwrap_or_default())
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        response.json().map_err(ApiError::from_reqwest)
    }

    fn push(&self, config: &SyncConfig, mutations: &[PushMutation]) -> ApiResult<Vec<PushResult>> {
        let client = reqwest::blocking::Client::new();
        let response = client
            .post(format!(
                "{}/api/sync/push",
                config.api_base_url.trim_end_matches('/')
            ))
            .bearer_auth(config.token.clone().unwrap_or_default())
            .json(&PushRequest { mutations })
            .send()
            .map_err(ApiError::from_reqwest)?
            .error_for_status()
            .map_err(ApiError::from_reqwest)?;
        let parsed: PushResponse = response.json().map_err(ApiError::from_reqwest)?;
        Ok(parsed.results)
    }
}
