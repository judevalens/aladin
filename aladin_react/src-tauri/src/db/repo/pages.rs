use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

use crate::db::{repo::SyncStatus, DbResult};

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct PageMetadataRow {
    pub id: String,
    pub title: String,
    pub revision: i64,
    pub updated_at: i64,
    pub sync_status: SyncStatus,
    pub version: i64,
}

pub struct PageRepo {
    dao: Arc<dyn PageDao>,
}

pub struct SqlitePageDao;

pub trait PageDao: Send + Sync {
    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<PageMetadataRow>>;
    fn upsert(&self, conn: &Connection, row: &PageMetadataRow) -> DbResult<()>;
}

impl PageRepo {
    pub fn new(dao: Arc<dyn PageDao>) -> Self {
        Self { dao }
    }

    pub fn get_metadata(&self, conn: &Connection, id: &str) -> DbResult<Option<PageMetadataRow>> {
        self.dao.get(conn, id)
    }

    pub fn upsert_metadata(&self, conn: &Connection, row: &PageMetadataRow) -> DbResult<()> {
        self.dao.upsert(conn, row)
    }
}

impl Default for PageRepo {
    fn default() -> Self {
        Self::new(Arc::new(SqlitePageDao))
    }
}

impl PageDao for SqlitePageDao {
    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<PageMetadataRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, title, revision, updated_at, sync_status, version FROM page_metadata WHERE id = ?1",
        )?;
        let mut rows = stmt.query_map(params![id], map_row)?;
        Ok(rows.next().transpose()?)
    }

    fn upsert(&self, conn: &Connection, row: &PageMetadataRow) -> DbResult<()> {
        conn.execute(
            "INSERT INTO page_metadata (id, title, revision, updated_at, sync_status, version) VALUES (?1, ?2, ?3, ?4, ?5, ?6)
             ON CONFLICT(id) DO UPDATE SET
                title=excluded.title,
                revision=excluded.revision,
                updated_at=excluded.updated_at,
                sync_status=excluded.sync_status,
                version=excluded.version",
            params![
                row.id,
                row.title,
                row.revision,
                row.updated_at,
                row.sync_status.as_str(),
                row.version
            ],
        )?;
        Ok(())
    }
}

fn map_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<PageMetadataRow> {
    Ok(PageMetadataRow {
        id: row.get(0)?,
        title: row.get(1)?,
        revision: row.get(2)?,
        updated_at: row.get(3)?,
        sync_status: SyncStatus::from_db(&row.get::<_, String>(4)?, 4)?,
        version: row.get(5)?,
    })
}
