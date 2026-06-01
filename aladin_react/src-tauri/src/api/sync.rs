use serde::{Deserialize, Serialize};

use crate::{
    api::{ApiError, ApiResult},
    sync::SyncConfig,
};

// Data-layer R1-C (client) — the sync pull client over the generic outbox.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Mirrors the server wire model (backend_v2/internal/service/sync_types.go).
// GET /api/sync/pull?since=<xid> returns FRAMES (one server txn each) since the
// client cursor, plus the new cursor (the log horizon) and a mode flag. The
// client applies each frame's entities under the per-entity `seq` guard into
// `nodes`. There is NO client push path — writes proxy to Go (see
// api/workspace_write.rs); the change returns as a frame.

/// uint64 wire values (xid cursor, per-entity seq) are JSON decimal STRINGs
/// (they can exceed the JS safe-integer range). This mirrors Go's
/// `json:",string"`.
pub(crate) mod string_u64 {
    use serde::{Deserialize, Deserializer, Serializer};

    pub fn serialize<S: Serializer>(value: &u64, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&value.to_string())
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(deserializer: D) -> Result<u64, D::Error> {
        let raw = String::deserialize(deserializer)?;
        raw.parse::<u64>().map_err(serde::de::Error::custom)
    }
}

/// One entity's change within a frame. `seq` is the per-entity version (the
/// staleness guard): apply iff `seq` > the stored seq. `data` carries the light
/// fields for an upsert (absent for a delete).
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FrameEntity {
    pub entity_kind: String,
    pub entity_id: String,
    #[serde(with = "string_u64")]
    pub seq: u64,
    pub op: String,
    #[serde(default)]
    pub data: Option<serde_json::Value>,
}

/// One server transaction's worth of changes — applied atomically by the client.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Frame {
    #[serde(default)]
    pub entities: Vec<FrameEntity>,
}

/// Pull response: frames since the cursor, the new cursor (log horizon), and the
/// mode (`delta` incremental merge, or `snapshot` cold-start full state).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncPullResult {
    #[serde(default)]
    pub frames: Vec<Frame>,
    #[serde(with = "string_u64")]
    pub cursor: u64,
    pub mode: String,
}

pub trait SyncApiClient: Send + Sync {
    /// Pulls frames with xid > `since` (0 = cold start → snapshot).
    fn pull(&self, config: &SyncConfig, since: u64) -> ApiResult<SyncPullResult>;
}

#[derive(Default)]
pub struct SyncApi;

impl SyncApiClient for SyncApi {
    fn pull(&self, config: &SyncConfig, since: u64) -> ApiResult<SyncPullResult> {
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
}
