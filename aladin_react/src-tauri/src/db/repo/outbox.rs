use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};

use crate::db::DbResult;

pub const MAX_OUTBOX_ATTEMPTS: i64 = 5;

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct OutboxMutationRow {
    pub id: String,
    pub entity_kind: String,
    pub entity_id: String,
    pub op_type: String,
    pub payload_json: String,
    pub mutation_id: String,
    pub attempts: i64,
    pub status: String,
    pub last_error: Option<String>,
    pub created_at: i64,
    pub updated_at: i64,
}

pub trait OutboxDao: Send + Sync {
    fn insert(&self, conn: &Connection, row: &OutboxMutationRow) -> DbResult<()>;
    fn list_pending(&self, conn: &Connection, limit: usize) -> DbResult<Vec<OutboxMutationRow>>;
    fn delete(&self, conn: &Connection, id: &str) -> DbResult<()>;
    fn record_retry(
        &self,
        conn: &Connection,
        id: &str,
        last_error: &str,
        updated_at: i64,
    ) -> DbResult<()>;
    fn mark_failed(
        &self,
        conn: &Connection,
        id: &str,
        last_error: &str,
        updated_at: i64,
    ) -> DbResult<()>;
}

pub struct SqliteOutboxDao;

impl OutboxDao for SqliteOutboxDao {
    fn insert(&self, conn: &Connection, row: &OutboxMutationRow) -> DbResult<()> {
        conn.execute(
            "INSERT INTO outbox_mutations
                (id, entity_kind, entity_id, op_type, payload_json, mutation_id, attempts, status, last_error, created_at, updated_at)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11)",
            params![
                row.id,
                row.entity_kind,
                row.entity_id,
                row.op_type,
                row.payload_json,
                row.mutation_id,
                row.attempts,
                row.status,
                row.last_error,
                row.created_at,
                row.updated_at,
            ],
        )?;
        Ok(())
    }

    fn list_pending(&self, conn: &Connection, limit: usize) -> DbResult<Vec<OutboxMutationRow>> {
        let mut stmt = conn.prepare(
            "SELECT id, entity_kind, entity_id, op_type, payload_json, mutation_id, attempts, status, last_error, created_at, updated_at
             FROM outbox_mutations
             WHERE status = 'PENDING' AND attempts < ?1
             ORDER BY created_at ASC
             LIMIT ?2",
        )?;
        let rows = stmt.query_map(params![MAX_OUTBOX_ATTEMPTS, limit as i64], map_row)?;
        rows.collect::<rusqlite::Result<Vec<_>>>()
            .map_err(Into::into)
    }

    fn delete(&self, conn: &Connection, id: &str) -> DbResult<()> {
        conn.execute("DELETE FROM outbox_mutations WHERE id = ?1", params![id])?;
        Ok(())
    }

    fn record_retry(
        &self,
        conn: &Connection,
        id: &str,
        last_error: &str,
        updated_at: i64,
    ) -> DbResult<()> {
        conn.execute(
            "UPDATE outbox_mutations
                SET attempts = attempts + 1,
                    last_error = ?2,
                    updated_at = ?3
              WHERE id = ?1",
            params![id, last_error, updated_at],
        )?;
        Ok(())
    }

    fn mark_failed(
        &self,
        conn: &Connection,
        id: &str,
        last_error: &str,
        updated_at: i64,
    ) -> DbResult<()> {
        conn.execute(
            "UPDATE outbox_mutations
                SET status = 'FAILED',
                    last_error = ?2,
                    updated_at = ?3
              WHERE id = ?1",
            params![id, last_error, updated_at],
        )?;
        Ok(())
    }
}

fn map_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<OutboxMutationRow> {
    Ok(OutboxMutationRow {
        id: row.get(0)?,
        entity_kind: row.get(1)?,
        entity_id: row.get(2)?,
        op_type: row.get(3)?,
        payload_json: row.get(4)?,
        mutation_id: row.get(5)?,
        attempts: row.get(6)?,
        status: row.get(7)?,
        last_error: row.get(8)?,
        created_at: row.get(9)?,
        updated_at: row.get(10)?,
    })
}
