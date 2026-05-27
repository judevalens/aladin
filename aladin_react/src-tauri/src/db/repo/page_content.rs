use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

use crate::db::{
    repo::{
        outbox::{self, OutboxDao, SqliteOutboxDao},
        SyncStatus,
    },
    Db, DbResult,
};

/// One row per page artifact; holds the BlockNote JSON blocks and the
/// optimistic-concurrency revision the backend cares about. Mirrors
/// `page_documents` in Postgres.
#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct PageContentRow {
    pub id: String,
    /// JSON-encoded array of BlockNote blocks. Opaque at this layer.
    pub blocks: String,
    pub revision: i64,
    pub updated_at: i64,
    pub sync_status: SyncStatus,
    pub version: i64,
}

/// Payload sent by the editor for a local-first save. `blocks` is the new
/// document, `revision` is the revision the client thinks is current
/// (used to detect divergence when the outbox replays against the server).
#[derive(Debug, Deserialize, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PageContentSaveInput {
    pub id: String,
    pub blocks: String,
    pub revision: i64,
    pub updated_at: i64,
    pub mutation_id: String,
}

pub trait PageContentDao: Send + Sync {
    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<PageContentRow>>;
    fn upsert(&self, conn: &Connection, row: &PageContentRow) -> DbResult<()>;
}

pub struct SqlitePageContentDao;

impl PageContentDao for SqlitePageContentDao {
    fn get(&self, conn: &Connection, id: &str) -> DbResult<Option<PageContentRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, blocks, revision, updated_at, sync_status, version
               FROM page_content WHERE id = ?1",
        )?;
        let mut rows = stmt.query_map(params![id], map_row)?;
        Ok(rows.next().transpose()?)
    }

    fn upsert(&self, conn: &Connection, row: &PageContentRow) -> DbResult<()> {
        conn.execute(
            "INSERT INTO page_content (id, blocks, revision, updated_at, sync_status, version)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)
             ON CONFLICT(id) DO UPDATE SET
                 blocks      = excluded.blocks,
                 revision    = excluded.revision,
                 updated_at  = excluded.updated_at,
                 sync_status = excluded.sync_status,
                 version     = excluded.version",
            params![
                row.id,
                row.blocks,
                row.revision,
                row.updated_at,
                row.sync_status.as_str(),
                row.version
            ],
        )?;
        Ok(())
    }
}

pub struct PageContentRepo {
    dao: Arc<dyn PageContentDao>,
    outbox: Arc<dyn OutboxDao>,
}

impl PageContentRepo {
    pub fn new(dao: Arc<dyn PageContentDao>, outbox: Arc<dyn OutboxDao>) -> Self {
        Self { dao, outbox }
    }

    pub fn get_content(&self, conn: &Connection, id: &str) -> DbResult<Option<PageContentRow>> {
        self.dao.get(conn, id)
    }

    /// Local-first save: write the new blocks to SQLite as PENDING and
    /// enqueue an outbox mutation. The outbox processor (M7.3) drives the
    /// remote PATCH /api/pages/{id} call when ready.
    pub fn save_local(&self, db: &Db, input: PageContentSaveInput) -> DbResult<PageContentRow> {
        let row = db.with_tx(|tx| {
            let next_version = self
                .dao
                .get(tx, &input.id)?
                .map(|r| r.version + 1)
                .unwrap_or(1);
            let row = PageContentRow {
                id: input.id.clone(),
                blocks: input.blocks.clone(),
                revision: input.revision,
                updated_at: input.updated_at,
                sync_status: SyncStatus::Pending,
                version: next_version,
            };
            self.dao.upsert(tx, &row)?;
            self.outbox.insert(
                tx,
                &outbox::OutboxMutationRow {
                    id: input.mutation_id.clone(),
                    entity_kind: "page_content".to_string(),
                    entity_id: input.id.clone(),
                    op_type: "update".to_string(),
                    payload_json: serde_json::to_string(&input)?,
                    mutation_id: input.mutation_id.clone(),
                    attempts: 0,
                    status: "PENDING".to_string(),
                    last_error: None,
                    created_at: input.updated_at,
                    updated_at: input.updated_at,
                },
            )?;
            Ok(row)
        })?;
        Ok(row)
    }
}

impl Default for PageContentRepo {
    fn default() -> Self {
        Self::new(Arc::new(SqlitePageContentDao), Arc::new(SqliteOutboxDao))
    }
}

fn map_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<PageContentRow> {
    Ok(PageContentRow {
        id: row.get(0)?,
        blocks: row.get(1)?,
        revision: row.get(2)?,
        updated_at: row.get(3)?,
        sync_status: SyncStatus::from_db(&row.get::<_, String>(4)?, 4)?,
        version: row.get(5)?,
    })
}
