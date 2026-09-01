use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::db::DbResult;
use crate::events::{DataEvent, EntityDeletedEvent};

// Data-layer — the local `reading_positions` read cache fed by sync FRAMES (entity kind
// "reading_position"). Same model as `watchlists`: a pure read cache, per-entity `seq`
// guard (apply iff incoming.seq > stored.seq), soft delete. The server is the only
// writer; the desktop's own PUT returns as a frame like everyone else's.
// `updated_at` is the SERVER's unix-ms stamp from the frame data (not a local clock),
// so the reader's newer-of(session, synced) comparison is consistent across devices.

/// The synced position for one document — id IS the artifact id.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ReadingPositionRow {
    pub id: String,
    pub page: i64,
    pub updated_at: i64,
}

/// The light fields a reading_position frame's `data` carries (matches the Go
/// lightReadingPositionData shape).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LightReadingPositionData {
    #[serde(default)]
    page: i64,
    #[serde(default)]
    updated_at: i64,
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

/// Last applied seq for a position, INCLUDING tombstones (0 if never seen).
pub fn stored_seq(conn: &Connection, id: &str) -> DbResult<i64> {
    Ok(conn
        .query_row(
            "SELECT seq FROM reading_positions WHERE id = ?1",
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
    let light: LightReadingPositionData =
        serde_json::from_value(value.clone()).map_err(light_err)?;
    conn.execute(
        "INSERT INTO reading_positions (id, page, seq, is_deleted, updated_at)
         VALUES (?1, ?2, ?3, 0, ?4)
         ON CONFLICT(id) DO UPDATE SET
            page = excluded.page,
            seq = excluded.seq,
            is_deleted = 0,
            updated_at = excluded.updated_at",
        params![id, light.page, seq, light.updated_at],
    )?;
    Ok(())
}

/// Applies one delete frame entity: soft delete (tombstone) under the seq guard.
pub fn apply_soft_delete(conn: &Connection, id: &str, seq: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO reading_positions (id, page, seq, is_deleted, updated_at)
         VALUES (?1, 1, ?2, 1, 0)
         ON CONFLICT(id) DO UPDATE SET
            seq = excluded.seq,
            is_deleted = 1",
        params![id, seq],
    )?;
    Ok(())
}

/// Apply one reading_position entity under the per-entity seq guard; returns the UI
/// event (None = stale/duplicate). Mirrors watchlists::apply.
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
    Ok(Some(match get_reading_position(conn, id)? {
        Some(row) => DataEvent::ReadingPositionUpserted(row),
        None => DataEvent::ReadingPositionDeleted(EntityDeletedEvent { id: id.to_string() }),
    }))
}

pub fn get_reading_position(conn: &Connection, id: &str) -> DbResult<Option<ReadingPositionRow>> {
    Ok(conn
        .query_row(
            "SELECT id, page, updated_at FROM reading_positions WHERE id = ?1 AND is_deleted = 0",
            params![id],
            |row| {
                Ok(ReadingPositionRow {
                    id: row.get(0)?,
                    page: row.get(1)?,
                    updated_at: row.get(2)?,
                })
            },
        )
        .optional()?)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::db::Db;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn test_db(name: &str) -> Db {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let path = std::env::temp_dir().join(format!(
            "aladin_readpos_test_{name}_{}_{}.sqlite",
            std::process::id(),
            nanos
        ));
        Db::open(path).unwrap()
    }

    fn data(page: i64, updated_at: i64) -> serde_json::Value {
        serde_json::json!({ "artifactId": "a1", "page": page, "updatedAt": updated_at })
    }

    #[test]
    fn applies_under_seq_guard_and_reads_back() {
        let db = test_db("guard");
        db.with_conn(|c| {
            let ev = apply(c, "a1", 1, "upsert", Some(&data(12, 1000)))?;
            assert!(matches!(ev, Some(DataEvent::ReadingPositionUpserted(_))));
            assert_eq!(get_reading_position(c, "a1")?.unwrap().page, 12);

            // Stale seq: skipped, no event, value untouched.
            assert!(apply(c, "a1", 1, "upsert", Some(&data(99, 2000)))?.is_none());
            let row = get_reading_position(c, "a1")?.unwrap();
            assert_eq!((row.page, row.updated_at), (12, 1000));

            // Newer seq wins (LWW).
            apply(c, "a1", 2, "upsert", Some(&data(87, 3000)))?;
            let row = get_reading_position(c, "a1")?.unwrap();
            assert_eq!((row.page, row.updated_at), (87, 3000));
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn tombstone_hides_and_blocks_resurrection() {
        let db = test_db("tombstone");
        db.with_conn(|c| {
            apply(c, "a1", 1, "upsert", Some(&data(5, 1000)))?;
            let ev = apply(c, "a1", 2, "delete", None)?;
            assert!(matches!(ev, Some(DataEvent::ReadingPositionDeleted(_))));
            assert!(get_reading_position(c, "a1")?.is_none());
            // A stale lower-seq upsert can't resurrect it.
            assert!(apply(c, "a1", 1, "upsert", Some(&data(5, 1000)))?.is_none());
            assert!(get_reading_position(c, "a1")?.is_none());
            Ok(())
        })
        .unwrap();
    }
}
