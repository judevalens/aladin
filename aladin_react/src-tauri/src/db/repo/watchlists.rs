use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::db::DbResult;
use crate::events::{DataEvent, EntityDeletedEvent};

// Data-layer — the local `watchlists` read cache fed by sync FRAMES (entity kind "watchlist").
// Same model as `nodes`/`signals`: a pure read cache, per-entity `seq` guard (apply iff
// incoming.seq > stored.seq), soft delete. The server is the only writer; changes arrive as
// frames. A watchlist is ONE entity — its members ride in items_json — so an add/remove is just an
// upsert of the whole list.

/// A watchlist rendered for the Markets switcher — maps 1:1 to the `watchlists` row the UI
/// consumes (tombstones are filtered out by reads). `item_count` is derived from items.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WatchlistRow {
    pub id: String,
    pub name: String,
    pub kind: String,
    pub position: i64,
    pub items: Vec<WatchlistItem>,
    pub item_count: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WatchlistItem {
    pub instrument_id: String,
    pub symbol: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub position: i64,
}

/// The light fields a watchlist frame's `data` carries (matches the Go lightWatchlistData shape).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LightWatchlistData {
    name: String,
    #[serde(default = "default_kind")]
    kind: String,
    #[serde(default)]
    position: i64,
    #[serde(default)]
    items: Vec<WatchlistItem>,
}

fn default_kind() -> String {
    "manual".to_string()
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

/// Last applied seq for a list, INCLUDING tombstones (0 if never seen).
pub fn stored_seq(conn: &Connection, id: &str) -> DbResult<i64> {
    Ok(conn
        .query_row(
            "SELECT seq FROM watchlists WHERE id = ?1",
            params![id],
            |row| row.get::<_, i64>(0),
        )
        .optional()?
        .unwrap_or(0))
}

/// Applies one upsert frame entity into the `watchlists` cache (caller passed the seq guard).
pub fn apply_upsert(
    conn: &Connection,
    id: &str,
    seq: i64,
    data: Option<&serde_json::Value>,
) -> DbResult<()> {
    let Some(value) = data else {
        return Ok(()); // upsert with no data can't populate columns; don't wipe
    };
    let light: LightWatchlistData = serde_json::from_value(value.clone()).map_err(light_err)?;
    let items_json = serde_json::to_string(&light.items).map_err(light_err)?;
    conn.execute(
        "INSERT INTO watchlists (id, name, kind, position, items_json, seq, is_deleted, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, 0, ?6)
         ON CONFLICT(id) DO UPDATE SET
            name = excluded.name,
            kind = excluded.kind,
            position = excluded.position,
            items_json = excluded.items_json,
            seq = excluded.seq,
            is_deleted = 0,
            updated_at = excluded.updated_at",
        params![id, light.name, light.kind, light.position, items_json, seq],
    )?;
    Ok(())
}

/// Applies one delete frame entity: soft delete (tombstone) under the seq guard.
pub fn apply_soft_delete(conn: &Connection, id: &str, seq: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO watchlists (id, name, seq, is_deleted, updated_at)
         VALUES (?1, '', ?2, 1, ?2)
         ON CONFLICT(id) DO UPDATE SET
            seq = excluded.seq,
            is_deleted = 1,
            updated_at = excluded.updated_at",
        params![id, seq],
    )?;
    Ok(())
}

/// Apply one watchlist entity under the per-entity seq guard; returns the UI event (None =
/// stale/duplicate). Mirrors signals::apply.
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
    Ok(Some(match get_watchlist(conn, id)? {
        Some(row) => DataEvent::WatchlistUpserted(row),
        None => DataEvent::WatchlistDeleted(EntityDeletedEvent { id: id.to_string() }),
    }))
}

fn row_to_watchlist(row: &rusqlite::Row) -> rusqlite::Result<WatchlistRow> {
    let items_json: Option<String> = row.get(3)?;
    let items: Vec<WatchlistItem> = items_json
        .and_then(|s| serde_json::from_str(&s).ok())
        .unwrap_or_default();
    let item_count = items.len() as i64;
    Ok(WatchlistRow {
        id: row.get(0)?,
        name: row.get(1)?,
        kind: row.get(2)?,
        items,
        item_count,
        position: row.get(4)?,
        updated_at: row.get(5)?,
    })
}

const WATCHLIST_COLS: &str = "id, name, kind, items_json, position, updated_at";

pub fn get_watchlist(conn: &Connection, id: &str) -> DbResult<Option<WatchlistRow>> {
    Ok(conn
        .query_row(
            &format!("SELECT {WATCHLIST_COLS} FROM watchlists WHERE id = ?1 AND is_deleted = 0"),
            params![id],
            row_to_watchlist,
        )
        .optional()?)
}

/// Lists live watchlists in display order (position, then recency).
pub fn list_watchlists(conn: &Connection) -> DbResult<Vec<WatchlistRow>> {
    let mut stmt = conn.prepare(&format!(
        "SELECT {WATCHLIST_COLS} FROM watchlists WHERE is_deleted = 0 ORDER BY position ASC, updated_at DESC"
    ))?;
    let rows = stmt.query_map([], row_to_watchlist)?;
    let mut out = Vec::new();
    for r in rows {
        out.push(r?);
    }
    Ok(out)
}
