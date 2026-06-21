use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::db::DbResult;
use crate::events::{DataEvent, EntityDeletedEvent};

// Data-layer — the local `signals` read cache fed by sync FRAMES (entity kind
// "signal"). Same model as `nodes`: a pure read cache, per-entity `seq` guard
// (apply iff incoming.seq > stored.seq), soft delete. The server is the only
// writer; changes arrive as frames. Claims live here (not `nodes`) so the
// workspace tree stays clean.

/// A claim rendered as a Signals card — maps 1:1 to the `signals` table row the
/// UI consumes (tombstones are filtered out by reads).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SignalRow {
    pub id: String,
    pub text: String,
    pub polarity: Option<String>,
    pub trust_tier: Option<String>,
    pub subjects: Vec<SignalSubjectRef>,
    pub assert_count: i64,
    pub deny_count: i64,
    pub supports: i64,
    pub contradicts: i64,
    pub qualifies: i64,
    pub signal_score: f64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SignalSubjectRef {
    pub id: String,
    pub name: String,
}

/// The light fields a signal frame's `data` carries (matches the Go
/// lightSignalData wire shape).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LightSignalData {
    text: String,
    polarity: Option<String>,
    trust_tier: Option<String>,
    #[serde(default)]
    subjects: Vec<SignalSubjectRef>,
    #[serde(default)]
    assert_count: i64,
    #[serde(default)]
    deny_count: i64,
    #[serde(default)]
    supports: i64,
    #[serde(default)]
    contradicts: i64,
    #[serde(default)]
    qualifies: i64,
    #[serde(default)]
    signal_score: f64,
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

/// Last applied seq for a signal, INCLUDING tombstones (0 if never seen).
pub fn stored_seq(conn: &Connection, id: &str) -> DbResult<i64> {
    Ok(conn
        .query_row("SELECT seq FROM signals WHERE id = ?1", params![id], |row| {
            row.get::<_, i64>(0)
        })
        .optional()?
        .unwrap_or(0))
}

/// Applies one upsert frame entity into the `signals` cache (caller passed the seq guard).
pub fn apply_upsert(
    conn: &Connection,
    id: &str,
    seq: i64,
    data: Option<&serde_json::Value>,
) -> DbResult<()> {
    let Some(value) = data else {
        return Ok(()); // upsert with no data can't populate columns; don't wipe
    };
    let light: LightSignalData = serde_json::from_value(value.clone()).map_err(light_err)?;
    let subjects_json = serde_json::to_string(&light.subjects).map_err(light_err)?;
    conn.execute(
        "INSERT INTO signals (id, text, polarity, trust_tier, subjects_json, assert_count, deny_count, supports, contradicts, qualifies, signal_score, seq, is_deleted, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, 0, ?12)
         ON CONFLICT(id) DO UPDATE SET
            text = excluded.text,
            polarity = excluded.polarity,
            trust_tier = excluded.trust_tier,
            subjects_json = excluded.subjects_json,
            assert_count = excluded.assert_count,
            deny_count = excluded.deny_count,
            supports = excluded.supports,
            contradicts = excluded.contradicts,
            qualifies = excluded.qualifies,
            signal_score = excluded.signal_score,
            seq = excluded.seq,
            is_deleted = 0,
            updated_at = excluded.updated_at",
        params![
            id,
            light.text,
            light.polarity,
            light.trust_tier,
            subjects_json,
            light.assert_count,
            light.deny_count,
            light.supports,
            light.contradicts,
            light.qualifies,
            light.signal_score,
            seq,
        ],
    )?;
    Ok(())
}

/// Applies one delete frame entity: soft delete (tombstone) under the seq guard.
pub fn apply_soft_delete(conn: &Connection, id: &str, seq: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO signals (id, text, seq, is_deleted, updated_at)
         VALUES (?1, '', ?2, 1, ?2)
         ON CONFLICT(id) DO UPDATE SET
            seq = excluded.seq,
            is_deleted = 1,
            updated_at = excluded.updated_at",
        params![id, seq],
    )?;
    Ok(())
}

/// Apply one signal entity under the per-entity seq guard; returns the UI event
/// (None = stale/duplicate). Mirrors nodes::apply.
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
    Ok(Some(match get_signal(conn, id)? {
        Some(row) => DataEvent::SignalUpserted(row),
        None => DataEvent::SignalDeleted(EntityDeletedEvent { id: id.to_string() }),
    }))
}

fn row_to_signal(row: &rusqlite::Row) -> rusqlite::Result<SignalRow> {
    let subjects_json: Option<String> = row.get(4)?;
    let subjects = subjects_json
        .and_then(|s| serde_json::from_str(&s).ok())
        .unwrap_or_default();
    Ok(SignalRow {
        id: row.get(0)?,
        text: row.get(1)?,
        polarity: row.get(2)?,
        trust_tier: row.get(3)?,
        subjects,
        assert_count: row.get(5)?,
        deny_count: row.get(6)?,
        supports: row.get(7)?,
        contradicts: row.get(8)?,
        qualifies: row.get(9)?,
        signal_score: row.get(10)?,
        updated_at: row.get(11)?,
    })
}

const SIGNAL_COLS: &str = "id, text, polarity, trust_tier, subjects_json, assert_count, deny_count, supports, contradicts, qualifies, signal_score, updated_at";

pub fn get_signal(conn: &Connection, id: &str) -> DbResult<Option<SignalRow>> {
    Ok(conn
        .query_row(
            &format!("SELECT {SIGNAL_COLS} FROM signals WHERE id = ?1 AND is_deleted = 0"),
            params![id],
            row_to_signal,
        )
        .optional()?)
}

/// Lists live signals. sort "top" → by signal_score desc; else by updated_at desc (recent).
pub fn list_signals(conn: &Connection, sort: &str) -> DbResult<Vec<SignalRow>> {
    let order = if sort == "top" {
        "signal_score DESC, updated_at DESC"
    } else {
        "updated_at DESC"
    };
    let mut stmt = conn.prepare(&format!(
        "SELECT {SIGNAL_COLS} FROM signals WHERE is_deleted = 0 ORDER BY {order} LIMIT 200"
    ))?;
    let rows = stmt.query_map([], row_to_signal)?;
    let mut out = Vec::new();
    for r in rows {
        out.push(r?);
    }
    Ok(out)
}
