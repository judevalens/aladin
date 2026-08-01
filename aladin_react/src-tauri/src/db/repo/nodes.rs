use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::db::DbResult;
use crate::events::{DataEvent, EntityDeletedEvent};

// Data-layer R1-C (client) — the `nodes` read cache fed by sync FRAMES.
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// `nodes` is a pure read cache. Frame entities apply under a per-ENTITY `seq`
// guard (apply iff incoming.seq > stored.seq); delete is a SOFT delete (the row
// is KEPT with is_deleted=1 so a stale lower-seq upsert can't resurrect it).
// Reads filter is_deleted=0. The client never writes locally or converges — the
// server is the only writer (writes proxy to Go), and the change returns as a
// frame. The cursor in sync_state is the uint64 xid (decimal string).

/// The pull cursor (last applied log horizon, a uint64 xid) — sync_state KV.
const CURSOR_KEY: &str = "workspace_cursor";

/// A node in the unified local workspace tree (folder or artifact). Maps 1:1 to
/// the `nodes` table; this is the read-cache row the UI consumes (tombstones are
/// filtered out by the read queries, so `is_deleted`/`seq` aren't exposed here).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NodeRow {
    pub id: String,
    pub kind: String,
    pub parent_id: Option<String>,
    pub position: i64,
    pub title: Option<String>,
    pub artifact_type: Option<String>,
    pub content: Option<String>,
    pub source_url: Option<String>,
    pub summary: Option<String>,
    pub metadata_json: Option<String>,
    /// Research folders only (RESEARCH_SURFACE_PRD §5): the extension row's LIGHT
    /// fields as the frame carried them — run state, exec mode, source kind. NULL on
    /// every other kind. The heavy strategy facts (hypothesis, manifest, universe,
    /// code hash) are fetched on demand, never synced onto the tree.
    pub research_json: Option<String>,
    pub updated_at: i64,
}

/// The light fields a frame's `data` carries for an upsert (doc §3) — what a
/// tree/list view needs. Heavy bodies (note content, page blocks) are NOT on the
/// wire; they are fetched on demand (deferred). `kind` is the entity kind
/// ("folder" | "artifact" | "research"); `type` maps to the artifact_type column.
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LightNodeData {
    kind: String,
    parent_id: Option<String>,
    #[serde(default)]
    position: i64,
    title: Option<String>,
    #[serde(rename = "type")]
    artifact_type: Option<String>,
    source_url: Option<String>,
    // Light-but-reactive artifact fields. Carried on the full payload like the
    // others (server SELECT + optimistic reconstruction), so a straight replace is
    // correct — no column-merge. `metadata` is a JSON object; stored as TEXT.
    #[serde(default)]
    summary: Option<String>,
    #[serde(default)]
    metadata: Option<serde_json::Value>,
    // Present only on research nodes — the strategy extension's light fields.
    #[serde(default)]
    research: Option<serde_json::Value>,
}

/// Last applied seq for an entity, INCLUDING tombstoned rows (0 if never seen).
/// Reading tombstones is load-bearing: a stale upsert arriving AFTER a delete is
/// rejected because the tombstone's seq is still here.
pub fn stored_seq(conn: &Connection, id: &str) -> DbResult<i64> {
    Ok(conn
        .query_row("SELECT seq FROM nodes WHERE id = ?1", params![id], |row| {
            row.get::<_, i64>(0)
        })
        .optional()?
        .unwrap_or(0))
}

/// Applies one upsert frame entity: writes the light columns, sets seq, clears
/// the tombstone. `content` is the one out-of-band column NOT touched (heavy, not
/// on the wire) so the cached note body survives; `summary`/`metadata` ARE on the
/// wire (full payload) and replaced straight. Caller has already passed the seq guard.
pub fn apply_upsert(
    conn: &Connection,
    id: &str,
    seq: i64,
    data: Option<&serde_json::Value>,
) -> DbResult<()> {
    let Some(value) = data else {
        // An upsert with no data can't populate columns; skip rather than wipe.
        return Ok(());
    };
    let light: LightNodeData = serde_json::from_value(value.clone()).map_err(|e| {
        crate::db::DbError::Sqlite(rusqlite::Error::FromSqlConversionFailure(
            0,
            rusqlite::types::Type::Text,
            Box::new(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                e.to_string(),
            )),
        ))
    })?;
    let metadata_json = light.metadata.as_ref().map(|v| v.to_string());
    let research_json = light.research.as_ref().map(|v| v.to_string());
    conn.execute(
        "INSERT INTO nodes (id, kind, parent_id, position, title, artifact_type, source_url, summary, metadata_json, research_json, seq, is_deleted, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, 0, ?11)
         ON CONFLICT(id) DO UPDATE SET
            kind = excluded.kind,
            parent_id = excluded.parent_id,
            position = excluded.position,
            title = excluded.title,
            artifact_type = excluded.artifact_type,
            source_url = excluded.source_url,
            summary = excluded.summary,
            metadata_json = excluded.metadata_json,
            research_json = excluded.research_json,
            seq = excluded.seq,
            is_deleted = 0,
            updated_at = excluded.updated_at",
        params![
            id,
            light.kind,
            light.parent_id,
            light.position,
            light.title,
            light.artifact_type,
            light.source_url,
            light.summary,
            metadata_json,
            research_json,
            seq,
        ],
    )?;
    Ok(())
}

/// Applies one delete frame entity: SOFT delete (tombstone) — keep the row, set
/// is_deleted=1 and the bumped seq, so a later stale lower-seq upsert is blocked.
/// If the row is absent, insert a tombstone stub (orphan delete-before-create).
pub fn apply_soft_delete(conn: &Connection, id: &str, kind: &str, seq: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO nodes (id, kind, position, seq, is_deleted, updated_at)
         VALUES (?1, ?2, 0, ?3, 1, ?3)
         ON CONFLICT(id) DO UPDATE SET
            seq = excluded.seq,
            is_deleted = 1,
            updated_at = excluded.updated_at",
        params![id, kind, seq],
    )?;
    Ok(())
}

/// Apply one entity to the cache under the per-ENTITY seq guard, and return the
/// UI event this repo chooses to dispatch (None = stale/duplicate — nothing
/// applied, nothing surfaced). The guard, the cache write, AND the dispatch
/// decision all live HERE with the data. Both the WS live/pull path (via the
/// engine's handler) and the local write path (db/repo/workspace_write.rs) call
/// this, so neither depends on the other and the engine never spills into a repo.
pub fn apply(
    conn: &Connection,
    kind: &str,
    id: &str,
    seq: i64,
    op: &str,
    data: Option<&serde_json::Value>,
) -> DbResult<Option<DataEvent>> {
    if seq <= stored_seq(conn, id)? {
        return Ok(None); // stale / duplicate / out-of-order — the seq guard (§3a)
    }
    match op {
        "delete" => apply_soft_delete(conn, id, kind, seq)?,
        _ => apply_upsert(conn, id, seq, data)?,
    }
    // The repo builds the event from the final committed state: a live row →
    // NodeUpserted; a tombstoned/absent row → NodeDeleted.
    Ok(Some(match get_node(conn, id)? {
        Some(row) => DataEvent::NodeUpserted(row),
        None => DataEvent::NodeDeleted(EntityDeletedEvent { id: id.to_string() }),
    }))
}

/// Reads the persisted pull cursor (0 if never set). uint64 xid, stored as text.
pub fn get_cursor(conn: &Connection) -> DbResult<u64> {
    let raw: Option<String> = conn
        .query_row(
            "SELECT value FROM sync_state WHERE key = ?1",
            params![CURSOR_KEY],
            |row| row.get(0),
        )
        .optional()?;
    Ok(raw.and_then(|s| s.parse::<u64>().ok()).unwrap_or(0))
}

/// Persists the pull cursor (the log horizon last pulled through).
pub fn set_cursor(conn: &Connection, cursor: u64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO sync_state (key, value) VALUES (?1, ?2)
         ON CONFLICT(key) DO UPDATE SET value = excluded.value",
        params![CURSOR_KEY, cursor.to_string()],
    )?;
    Ok(())
}

/// All LIVE nodes (is_deleted=0), flat, ordered by (position, insertion). The
/// tree is materialized from this by the read layer.
pub fn list_nodes(conn: &Connection) -> DbResult<Vec<NodeRow>> {
    let mut stmt = conn.prepare(
        "SELECT id, kind, parent_id, position, title, artifact_type, content, source_url, summary, metadata_json, research_json, updated_at
         FROM nodes WHERE is_deleted = 0 ORDER BY position ASC, rowid ASC",
    )?;
    let rows = stmt.query_map([], map_node_row)?;
    rows.collect::<rusqlite::Result<Vec<_>>>()
        .map_err(Into::into)
}

/// One LIVE node by id (tombstoned rows read as absent).
pub fn get_node(conn: &Connection, id: &str) -> DbResult<Option<NodeRow>> {
    let mut stmt = conn.prepare(
        "SELECT id, kind, parent_id, position, title, artifact_type, content, source_url, summary, metadata_json, research_json, updated_at
         FROM nodes WHERE id = ?1 AND is_deleted = 0",
    )?;
    let mut rows = stmt.query_map(params![id], map_node_row)?;
    Ok(rows.next().transpose()?)
}

/// Next sibling position under `parent_id` (max+1 over LIVE rows), used by the
/// cache-only upserts (read-through caching of a fetched artifact).
pub fn next_position(conn: &Connection, parent_id: Option<&str>) -> DbResult<i64> {
    let next: i64 = match parent_id {
        Some(p) => conn.query_row(
            "SELECT COALESCE(MAX(position), 0) + 1 FROM nodes WHERE parent_id = ?1 AND is_deleted = 0",
            params![p],
            |r| r.get(0),
        )?,
        None => conn.query_row(
            "SELECT COALESCE(MAX(position), 0) + 1 FROM nodes WHERE parent_id IS NULL AND is_deleted = 0",
            [],
            |r| r.get(0),
        )?,
    };
    Ok(next)
}

/// Cache-only upsert of a full node row (NO sync semantics) — used to cache a
/// server/API read (e.g. read-through of an artifact body) into the local model.
/// Preserves the row's seq/is_deleted (does not touch the sync columns).
pub fn upsert_local(conn: &Connection, row: &NodeRow) -> DbResult<()> {
    conn.execute(
        "INSERT INTO nodes (id, kind, parent_id, position, title, artifact_type, content, source_url, summary, metadata_json, research_json, updated_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)
         ON CONFLICT(id) DO UPDATE SET
            kind = excluded.kind,
            parent_id = excluded.parent_id,
            position = excluded.position,
            title = excluded.title,
            artifact_type = excluded.artifact_type,
            content = excluded.content,
            source_url = excluded.source_url,
            summary = excluded.summary,
            metadata_json = excluded.metadata_json,
            research_json = excluded.research_json,
            updated_at = excluded.updated_at",
        params![
            row.id,
            row.kind,
            row.parent_id,
            row.position,
            row.title,
            row.artifact_type,
            row.content,
            row.source_url,
            row.summary,
            row.metadata_json,
            row.research_json,
            row.updated_at,
        ],
    )?;
    Ok(())
}

/// REPLACE reconcile for a cold-start snapshot (doc §6): remove any local row
/// whose id is NOT in the snapshot's id set. The snapshot is the server's
/// complete authoritative set (it INCLUDES tombstones), so a local row absent
/// from it is stale cache garbage. Returns the removed ids (for UI events). A
/// concurrent live create newer than the snapshot self-heals on the next delta
/// pull (its xid > the snapshot horizon).
pub fn retain_only(conn: &Connection, keep_ids: &[String]) -> DbResult<Vec<DataEvent>> {
    // Find live rows not in the keep set (read before delete, for events).
    let keep: std::collections::HashSet<&str> = keep_ids.iter().map(|s| s.as_str()).collect();
    let mut stmt = conn.prepare("SELECT id FROM nodes WHERE is_deleted = 0")?;
    let live_ids = stmt
        .query_map([], |row| row.get::<_, String>(0))?
        .collect::<rusqlite::Result<Vec<_>>>()?;
    let mut removed = Vec::new();
    for id in live_ids {
        if !keep.contains(id.as_str()) {
            conn.execute("DELETE FROM nodes WHERE id = ?1", params![id])?;
            // The repo dispatches its own deletes (snapshot REPLACE drops garbage).
            removed.push(DataEvent::NodeDeleted(EntityDeletedEvent { id }));
        }
    }
    Ok(removed)
}

fn map_node_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<NodeRow> {
    Ok(NodeRow {
        id: row.get(0)?,
        kind: row.get(1)?,
        parent_id: row.get(2)?,
        position: row.get(3)?,
        title: row.get(4)?,
        artifact_type: row.get(5)?,
        content: row.get(6)?,
        source_url: row.get(7)?,
        summary: row.get(8)?,
        metadata_json: row.get(9)?,
        research_json: row.get(10)?,
        updated_at: row.get(11)?,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::db::Db;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn test_db(name: &str) -> Db {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let path = std::env::temp_dir().join(format!(
            "aladin_nodes_test_{name}_{}_{}.sqlite",
            std::process::id(),
            nanos
        ));
        Db::open(PathBuf::from(path)).unwrap()
    }

    fn folder_data(id: &str, title: &str) -> serde_json::Value {
        serde_json::json!({
            "id": id, "kind": "folder", "parentId": null, "position": 1, "title": title
        })
    }

    #[test]
    fn upsert_sets_seq_and_clears_tombstone() {
        let db = test_db("upsert_seq");
        db.with_conn(|c| {
            apply_upsert(c, "f1", 5, Some(&folder_data("f1", "Docs")))?;
            assert_eq!(stored_seq(c, "f1")?, 5);
            assert_eq!(get_node(c, "f1")?.unwrap().title.as_deref(), Some("Docs"));
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn soft_delete_tombstones_and_blocks_stale_upsert() {
        let db = test_db("soft_delete_block");
        db.with_conn(|c| {
            apply_upsert(c, "f1", 1, Some(&folder_data("f1", "Docs")))?;
            apply_soft_delete(c, "f1", "folder", 2)?;
            // Tombstoned: read as absent, but seq retained.
            assert!(get_node(c, "f1")?.is_none());
            assert_eq!(stored_seq(c, "f1")?, 2);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn higher_seq_upsert_resurrects_after_delete() {
        let db = test_db("resurrect");
        db.with_conn(|c| {
            apply_soft_delete(c, "f1", "folder", 2)?;
            // A NEWER upsert (seq 3 > 2) clears the tombstone.
            apply_upsert(c, "f1", 3, Some(&folder_data("f1", "Back")))?;
            assert_eq!(get_node(c, "f1")?.unwrap().title.as_deref(), Some("Back"));
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn cursor_roundtrip_u64() {
        let db = test_db("cursor_u64");
        db.with_conn(|c| {
            assert_eq!(get_cursor(c)?, 0);
            let big: u64 = 9_223_372_036_854_775_810; // > i64::MAX
            set_cursor(c, big)?;
            assert_eq!(get_cursor(c)?, big);
            Ok(())
        })
        .unwrap();
    }
}
