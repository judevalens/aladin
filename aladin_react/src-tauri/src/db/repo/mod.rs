pub mod artifacts;
pub mod browser;
pub mod nodes;
pub mod pages;
pub mod signals;
pub mod watchlists;
pub mod workspace;

use rusqlite::types::Type;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum SyncStatus {
    Pending,
    Synced,
    Failed,
}

impl SyncStatus {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Pending => "PENDING",
            Self::Synced => "SYNCED",
            Self::Failed => "FAILED",
        }
    }

    pub fn from_db(value: &str, col: usize) -> rusqlite::Result<Self> {
        match value {
            "PENDING" => Ok(Self::Pending),
            "SYNCED" => Ok(Self::Synced),
            "FAILED" => Ok(Self::Failed),
            _ => Err(rusqlite::Error::FromSqlConversionFailure(
                col,
                Type::Text,
                format!("invalid sync status: {value}").into(),
            )),
        }
    }
}
