use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::db::DbResult;
use crate::events::{DataEvent, EntityDeletedEvent};

// Shard local state — the local `shard_kv` read cache fed by sync FRAMES (entity
// kind "shard_kv", one entity PER KEY, id "<shard_id>#<key>"). Same model as
// `nodes`/`watchlists`: pure read cache, per-entity seq guard, soft delete. Only
// the PUBLISHED channel syncs (draft is the agent's server-side sandbox), so
// every row here is published-channel by construction. The shard's own writes go
// host bridge → REST → Go; the mutation returns as a frame that lands here, and
// the host pushes the change into subscribed iframes (SHARD_LOCAL_STATE.md).

/// One shard-local key as the UI/bridge consumes it (tombstones filtered by reads).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ShardKvRow {
    pub id: String,
    pub shard_id: String,
    pub key: String,
    pub value: serde_json::Value,
    pub revision: i64,
    pub updated_at: i64,
}

/// The light fields a shard_kv frame's `data` carries (matches Go lightShardKVData).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LightShardKvData {
    shard_id: String,
    key: String,
    #[serde(default)]
    value: serde_json::Value,
    #[serde(default)]
    revision: i64,
}

fn light_err(e: impl std::fmt::Display) -> crate::db::DbError {
    crate::db::DbError::Sqlite(rusqlite::Error::FromSqlConversionFailure(
        0,
        rusqlite::types::Type::Text,
        Box::new(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            e.to_string(),
        )),
    ))
}

/// Last applied seq for an entity id, INCLUDING tombstones (0 if never seen).
pub fn stored_seq(conn: &Connection, id: &str) -> DbResult<i64> {
    Ok(conn
        .query_row(
            "SELECT seq FROM shard_kv WHERE id = ?1",
            params![id],
            |row| row.get::<_, i64>(0),
        )
        .optional()?
        .unwrap_or(0))
}

/// Applies one upsert frame entity (caller passed the seq guard).
pub fn apply_upsert(
    conn: &Connection,
    id: &str,
    seq: i64,
    data: Option<&serde_json::Value>,
) -> DbResult<()> {
    let Some(value) = data else {
        return Ok(()); // upsert with no data can't populate columns; don't wipe
    };
    let light: LightShardKvData = serde_json::from_value(value.clone()).map_err(light_err)?;
    let value_json = serde_json::to_string(&light.value).map_err(light_err)?;
    conn.execute(
        "INSERT INTO shard_kv (id, shard_id, key, value_json, revision, seq, is_deleted, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, 0, ?6)
         ON CONFLICT(id) DO UPDATE SET
            shard_id = excluded.shard_id,
            key = excluded.key,
            value_json = excluded.value_json,
            revision = excluded.revision,
            seq = excluded.seq,
            is_deleted = 0,
            updated_at = excluded.updated_at",
        params![id, light.shard_id, light.key, value_json, light.revision, seq],
    )?;
    Ok(())
}

/// Applies one delete frame entity: soft delete (tombstone) under the seq guard.
/// A tombstone for a never-seen id still records (id, seq) so a stale lower-seq
/// upsert can't resurrect it; shard_id/key are best-effort parsed from the id.
pub fn apply_soft_delete(conn: &Connection, id: &str, seq: i64) -> DbResult<()> {
    let (shard_id, key) = id.split_once('#').unwrap_or((id, ""));
    conn.execute(
        "INSERT INTO shard_kv (id, shard_id, key, value_json, revision, seq, is_deleted, updated_at)
         VALUES (?1, ?2, ?3, NULL, 0, ?4, 1, ?4)
         ON CONFLICT(id) DO UPDATE SET
            seq = excluded.seq,
            is_deleted = 1,
            updated_at = excluded.updated_at",
        params![id, shard_id, key, seq],
    )?;
    Ok(())
}

/// Apply one shard_kv entity under the per-entity seq guard; returns the UI event
/// (None = stale/duplicate). Mirrors watchlists::apply.
pub fn apply(
    conn: &Connection,
    id: &str,
    seq: i64,
    op: &str,
    data: Option<&serde_json::Value>,
) -> DbResult<Option<DataEvent>> {
    if seq <= stored_seq(conn, id)? {
        return Ok(None); // seq guard
    }
    match op {
        "delete" => apply_soft_delete(conn, id, seq)?,
        _ => apply_upsert(conn, id, seq, data)?,
    }
    Ok(Some(match get_entry(conn, id)? {
        Some(row) => DataEvent::ShardKvUpserted(row),
        None => DataEvent::ShardKvDeleted(EntityDeletedEvent { id: id.to_string() }),
    }))
}

fn row_to_entry(row: &rusqlite::Row) -> rusqlite::Result<ShardKvRow> {
    let value_json: Option<String> = row.get(3)?;
    let value = value_json
        .and_then(|s| serde_json::from_str(&s).ok())
        .unwrap_or(serde_json::Value::Null);
    Ok(ShardKvRow {
        id: row.get(0)?,
        shard_id: row.get(1)?,
        key: row.get(2)?,
        value,
        revision: row.get(4)?,
        updated_at: row.get(5)?,
    })
}

const SHARD_KV_COLS: &str = "id, shard_id, key, value_json, revision, updated_at";

pub fn get_entry(conn: &Connection, id: &str) -> DbResult<Option<ShardKvRow>> {
    Ok(conn
        .query_row(
            &format!("SELECT {SHARD_KV_COLS} FROM shard_kv WHERE id = ?1 AND is_deleted = 0"),
            params![id],
            row_to_entry,
        )
        .optional()?)
}

/// Lists a shard's live keys under a prefix ("" = all), ordered by key — the
/// bridge's kv.list/kv.subscribe seed read.
pub fn list_prefix(conn: &Connection, shard_id: &str, prefix: &str) -> DbResult<Vec<ShardKvRow>> {
    let like = format!(
        "{}%",
        prefix
            .replace('\\', "\\\\")
            .replace('%', "\\%")
            .replace('_', "\\_")
    );
    let mut stmt = conn.prepare(&format!(
        "SELECT {SHARD_KV_COLS} FROM shard_kv
          WHERE shard_id = ?1 AND key LIKE ?2 ESCAPE '\\' AND is_deleted = 0
          ORDER BY key ASC"
    ))?;
    let rows = stmt.query_map(params![shard_id, like], row_to_entry)?;
    let mut out = Vec::new();
    for r in rows {
        out.push(r?);
    }
    Ok(out)
}
