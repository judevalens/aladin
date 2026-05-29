use rusqlite::{params, Connection, OptionalExtension};
use serde::{Deserialize, Serialize};

use crate::api::sync::SyncChange;
use crate::db::DbResult;

// Data-layer redesign, Phase A (client) — apply the workspace change feed into
// the clean `nodes` tree, guarded per (entity, field) by the newest-wins seq.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// The feed is server-authoritative and seq-ordered. We apply each change iff
// its seq is strictly newer than the last seq we applied for that
// (entity_kind, entity_id, field); create/delete share the entity-level
// existence slot (field = ""). Application is idempotent: replaying a delta is
// a no-op, and out-of-order/duplicate changes can't regress a field.

/// The reconnect/replay cursor (max applied feed seq) — lives in sync_state KV.
const CURSOR_KEY: &str = "workspace_cursor";

/// Field key for entity-level (create/delete) changes in `field_seq`. Mirrors
/// the server's COALESCE(field,'') coalescing slot.
const EXISTENCE_FIELD: &str = "";

/// A node in the unified local workspace tree (folder or artifact). Maps 1:1 to
/// the `nodes` table and the sync feed's per-field columns.
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
    pub updated_at: i64,
}

/// What `apply_change` did, so the caller can emit the right UI data event.
#[derive(Debug, Clone, PartialEq)]
pub enum ApplyOutcome {
    /// The node row was created or a field updated; carries the node id.
    Upserted(String),
    /// The node row was removed; carries the node id.
    Deleted(String),
    /// Stale/duplicate/unknown — nothing changed.
    Skipped,
}

/// The `nodes` columns a feed field can target. Restricting to this closed set
/// keeps the dynamic column name in `apply_field` a compile-time constant (no
/// SQL injection surface from the wire `field` string).
#[derive(Debug, Clone, Copy)]
enum NodeColumn {
    Title,
    ParentId,
    Position,
    ArtifactType,
    Content,
    SourceUrl,
    Summary,
    Metadata,
}

/// Maps a feed field name to its `nodes` column. Unknown fields return None and
/// are skipped (but their seq is still recorded so we don't reprocess them).
fn map_field_to_column(field: &str) -> Option<NodeColumn> {
    match field {
        "title" => Some(NodeColumn::Title),
        "parentId" => Some(NodeColumn::ParentId),
        "position" => Some(NodeColumn::Position),
        "type" => Some(NodeColumn::ArtifactType),
        "content" => Some(NodeColumn::Content),
        "sourceUrl" => Some(NodeColumn::SourceUrl),
        "summary" => Some(NodeColumn::Summary),
        "metadata" => Some(NodeColumn::Metadata),
        _ => None,
    }
}

/// JSON string value → owned String; null/absent/non-string → None.
fn value_as_opt_text(value: Option<&serde_json::Value>) -> Option<String> {
    match value {
        Some(serde_json::Value::String(s)) => Some(s.clone()),
        _ => None,
    }
}

/// JSON number value → i64; anything else → 0.
fn value_as_i64(value: Option<&serde_json::Value>) -> i64 {
    value.and_then(|v| v.as_i64()).unwrap_or(0)
}

/// JSON value → its serialized text for storage; null/absent → None.
fn value_as_json_text(value: Option<&serde_json::Value>) -> Option<String> {
    match value {
        None | Some(serde_json::Value::Null) => None,
        Some(v) => Some(v.to_string()),
    }
}

/// Last applied seq for an (entity, field) slot, or None if never applied.
fn stored_seq(conn: &Connection, kind: &str, id: &str, field: &str) -> DbResult<Option<i64>> {
    Ok(conn
        .query_row(
            "SELECT seq FROM field_seq WHERE entity_kind = ?1 AND entity_id = ?2 AND field = ?3",
            params![kind, id, field],
            |row| row.get(0),
        )
        .optional()?)
}

/// Records `seq` as the newest applied for the (entity, field) slot.
fn bump_seq(conn: &Connection, kind: &str, id: &str, field: &str, seq: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO field_seq (entity_kind, entity_id, field, seq) VALUES (?1, ?2, ?3, ?4)
         ON CONFLICT(entity_kind, entity_id, field) DO UPDATE SET seq = excluded.seq",
        params![kind, id, field, seq],
    )?;
    Ok(())
}

/// Inserts a stub node row if absent (orphan tolerance: a field change may
/// arrive before its create within live delivery). No-op if it already exists.
fn ensure_node(conn: &Connection, id: &str, kind: &str, seq: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO nodes (id, kind, position, updated_at) VALUES (?1, ?2, 0, ?3)
         ON CONFLICT(id) DO NOTHING",
        params![id, kind, seq],
    )?;
    Ok(())
}

/// Applies one field update to the node's column, ensuring the row exists.
fn apply_field(
    conn: &Connection,
    kind: &str,
    id: &str,
    column: NodeColumn,
    change: &SyncChange,
) -> DbResult<()> {
    ensure_node(conn, id, kind, change.seq)?;
    let value = change.value.as_ref();
    match column {
        NodeColumn::Title => conn.execute(
            "UPDATE nodes SET title = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_opt_text(value), change.seq],
        )?,
        NodeColumn::ParentId => conn.execute(
            "UPDATE nodes SET parent_id = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_opt_text(value), change.seq],
        )?,
        NodeColumn::Position => conn.execute(
            "UPDATE nodes SET position = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_i64(value), change.seq],
        )?,
        NodeColumn::ArtifactType => conn.execute(
            "UPDATE nodes SET artifact_type = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_opt_text(value), change.seq],
        )?,
        NodeColumn::Content => conn.execute(
            "UPDATE nodes SET content = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_opt_text(value), change.seq],
        )?,
        NodeColumn::SourceUrl => conn.execute(
            "UPDATE nodes SET source_url = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_opt_text(value), change.seq],
        )?,
        NodeColumn::Summary => conn.execute(
            "UPDATE nodes SET summary = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_opt_text(value), change.seq],
        )?,
        NodeColumn::Metadata => conn.execute(
            "UPDATE nodes SET metadata_json = ?2, updated_at = ?3 WHERE id = ?1",
            params![id, value_as_json_text(value), change.seq],
        )?,
    };
    Ok(())
}

/// Applies one feed change to `nodes`, guarded by the per-(entity,field)
/// newest-wins seq. Idempotent: a stale or already-applied change is skipped.
/// Caller runs this inside a transaction and advances the cursor afterward.
pub fn apply_change(conn: &Connection, change: &SyncChange) -> DbResult<ApplyOutcome> {
    let kind = change.entity_kind.as_str();
    let id = change.entity_id.as_str();

    match change.op.as_str() {
        "create" => {
            if !seq_is_newer(conn, kind, id, EXISTENCE_FIELD, change.seq)? {
                return Ok(ApplyOutcome::Skipped);
            }
            conn.execute(
                "INSERT INTO nodes (id, kind, position, updated_at) VALUES (?1, ?2, 0, ?3)
                 ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, updated_at = excluded.updated_at",
                params![id, kind, change.seq],
            )?;
            bump_seq(conn, kind, id, EXISTENCE_FIELD, change.seq)?;
            Ok(ApplyOutcome::Upserted(id.to_string()))
        }
        "delete" => {
            if !seq_is_newer(conn, kind, id, EXISTENCE_FIELD, change.seq)? {
                return Ok(ApplyOutcome::Skipped);
            }
            conn.execute("DELETE FROM nodes WHERE id = ?1", params![id])?;
            bump_seq(conn, kind, id, EXISTENCE_FIELD, change.seq)?;
            Ok(ApplyOutcome::Deleted(id.to_string()))
        }
        "update" => {
            let Some(field) = change.field.as_deref() else {
                return Ok(ApplyOutcome::Skipped);
            };
            if !seq_is_newer(conn, kind, id, field, change.seq)? {
                return Ok(ApplyOutcome::Skipped);
            }
            // Record the seq regardless, so unknown fields aren't reprocessed.
            let outcome = match map_field_to_column(field) {
                Some(column) => {
                    apply_field(conn, kind, id, column, change)?;
                    ApplyOutcome::Upserted(id.to_string())
                }
                None => ApplyOutcome::Skipped,
            };
            bump_seq(conn, kind, id, field, change.seq)?;
            Ok(outcome)
        }
        _ => Ok(ApplyOutcome::Skipped),
    }
}

/// True if `incoming` is strictly newer than the last applied seq for the slot.
fn seq_is_newer(
    conn: &Connection,
    kind: &str,
    id: &str,
    field: &str,
    incoming: i64,
) -> DbResult<bool> {
    Ok(match stored_seq(conn, kind, id, field)? {
        Some(stored) => incoming > stored,
        None => true,
    })
}

/// Reads the persisted pull cursor (0 if never set).
pub fn get_cursor(conn: &Connection) -> DbResult<i64> {
    let raw: Option<String> = conn
        .query_row(
            "SELECT value FROM sync_state WHERE key = ?1",
            params![CURSOR_KEY],
            |row| row.get(0),
        )
        .optional()?;
    Ok(raw.and_then(|s| s.parse::<i64>().ok()).unwrap_or(0))
}

/// Persists the pull cursor (max applied feed seq).
pub fn set_cursor(conn: &Connection, cursor: i64) -> DbResult<()> {
    conn.execute(
        "INSERT INTO sync_state (key, value) VALUES (?1, ?2)
         ON CONFLICT(key) DO UPDATE SET value = excluded.value",
        params![CURSOR_KEY, cursor.to_string()],
    )?;
    Ok(())
}

/// All nodes, flat, ordered by (position, insertion). The tree is materialized
/// from this by the read layer (orphan-tolerant + cycle-breaking).
pub fn list_nodes(conn: &Connection) -> DbResult<Vec<NodeRow>> {
    let mut stmt = conn.prepare(
        "SELECT id, kind, parent_id, position, title, artifact_type, content, source_url, summary, metadata_json, updated_at
         FROM nodes ORDER BY position ASC, rowid ASC",
    )?;
    let rows = stmt.query_map([], map_node_row)?;
    rows.collect::<rusqlite::Result<Vec<_>>>().map_err(Into::into)
}

/// One node by id.
pub fn get_node(conn: &Connection, id: &str) -> DbResult<Option<NodeRow>> {
    let mut stmt = conn.prepare(
        "SELECT id, kind, parent_id, position, title, artifact_type, content, source_url, summary, metadata_json, updated_at
         FROM nodes WHERE id = ?1",
    )?;
    let mut rows = stmt.query_map(params![id], map_node_row)?;
    Ok(rows.next().transpose()?)
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
        updated_at: row.get(10)?,
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

    fn update(kind: &str, id: &str, field: &str, value: serde_json::Value, seq: i64) -> SyncChange {
        SyncChange {
            seq,
            entity_kind: kind.to_string(),
            entity_id: id.to_string(),
            op: "update".to_string(),
            field: Some(field.to_string()),
            value: Some(value),
            mutation_id: None,
        }
    }

    fn entity(kind: &str, id: &str, op: &str, seq: i64) -> SyncChange {
        SyncChange {
            seq,
            entity_kind: kind.to_string(),
            entity_id: id.to_string(),
            op: op.to_string(),
            field: None,
            value: None,
            mutation_id: None,
        }
    }

    #[test]
    fn create_then_fields_builds_node() {
        let db = test_db("create_then_fields");
        db.with_conn(|c| {
            assert_eq!(
                apply_change(c, &entity("artifact", "a1", "create", 1))?,
                ApplyOutcome::Upserted("a1".into())
            );
            apply_change(c, &update("artifact", "a1", "title", serde_json::json!("Hello"), 2))?;
            apply_change(c, &update("artifact", "a1", "type", serde_json::json!("note"), 3))?;
            apply_change(c, &update("artifact", "a1", "parentId", serde_json::json!("f1"), 4))?;
            apply_change(c, &update("artifact", "a1", "position", serde_json::json!(7), 5))?;
            let node = get_node(c, "a1")?.expect("node exists");
            assert_eq!(node.kind, "artifact");
            assert_eq!(node.title.as_deref(), Some("Hello"));
            assert_eq!(node.artifact_type.as_deref(), Some("note"));
            assert_eq!(node.parent_id.as_deref(), Some("f1"));
            assert_eq!(node.position, 7);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn seq_guard_rejects_stale_field() {
        let db = test_db("seq_guard_stale");
        db.with_conn(|c| {
            apply_change(c, &entity("folder", "f1", "create", 1))?;
            apply_change(c, &update("folder", "f1", "title", serde_json::json!("B"), 5))?;
            // A stale rename (seq 3 < 5) must not regress the title.
            let outcome =
                apply_change(c, &update("folder", "f1", "title", serde_json::json!("A"), 3))?;
            assert_eq!(outcome, ApplyOutcome::Skipped);
            assert_eq!(get_node(c, "f1")?.unwrap().title.as_deref(), Some("B"));
            // A newer rename (seq 9 > 5) applies.
            apply_change(c, &update("folder", "f1", "title", serde_json::json!("C"), 9))?;
            assert_eq!(get_node(c, "f1")?.unwrap().title.as_deref(), Some("C"));
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn delete_removes_node_and_blocks_stale_recreate() {
        let db = test_db("delete_blocks_recreate");
        db.with_conn(|c| {
            apply_change(c, &entity("artifact", "a1", "create", 1))?;
            apply_change(c, &update("artifact", "a1", "title", serde_json::json!("X"), 2))?;
            assert_eq!(
                apply_change(c, &entity("artifact", "a1", "delete", 10))?,
                ApplyOutcome::Deleted("a1".into())
            );
            assert!(get_node(c, "a1")?.is_none());
            // A stale re-create (seq 5 < 10) must stay rejected.
            assert_eq!(
                apply_change(c, &entity("artifact", "a1", "create", 5))?,
                ApplyOutcome::Skipped
            );
            assert!(get_node(c, "a1")?.is_none());
            // A genuinely newer re-create (seq 20 > 10) brings it back.
            apply_change(c, &entity("artifact", "a1", "create", 20))?;
            assert!(get_node(c, "a1")?.is_some());
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn parent_id_null_sets_root() {
        let db = test_db("parent_null_root");
        db.with_conn(|c| {
            apply_change(c, &entity("folder", "f1", "create", 1))?;
            apply_change(c, &update("folder", "f1", "parentId", serde_json::json!("p1"), 2))?;
            assert_eq!(get_node(c, "f1")?.unwrap().parent_id.as_deref(), Some("p1"));
            // parentId → null moves it to the root.
            apply_change(c, &update("folder", "f1", "parentId", serde_json::Value::Null, 3))?;
            assert_eq!(get_node(c, "f1")?.unwrap().parent_id, None);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn metadata_stored_as_json_text() {
        let db = test_db("metadata_json");
        db.with_conn(|c| {
            apply_change(c, &entity("artifact", "a1", "create", 1))?;
            apply_change(
                c,
                &update("artifact", "a1", "metadata", serde_json::json!({"k": "v"}), 2),
            )?;
            let stored = get_node(c, "a1")?.unwrap().metadata_json.unwrap();
            let parsed: serde_json::Value = serde_json::from_str(&stored).unwrap();
            assert_eq!(parsed, serde_json::json!({"k": "v"}));
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn cursor_roundtrips() {
        let db = test_db("cursor_roundtrip");
        db.with_conn(|c| {
            assert_eq!(get_cursor(c)?, 0);
            set_cursor(c, 42)?;
            assert_eq!(get_cursor(c)?, 42);
            set_cursor(c, 100)?;
            assert_eq!(get_cursor(c)?, 100);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn field_before_create_is_orphan_tolerant() {
        let db = test_db("orphan_field");
        db.with_conn(|c| {
            // A field update arrives before the create (out-of-order live event):
            // a stub row is created so the value isn't lost.
            apply_change(c, &update("folder", "f9", "title", serde_json::json!("Early"), 5))?;
            let node = get_node(c, "f9")?.expect("stub created");
            assert_eq!(node.title.as_deref(), Some("Early"));
            assert_eq!(node.kind, "folder");
            // The later create (lower seq) reconciles existence without clobbering.
            apply_change(c, &entity("folder", "f9", "create", 1))?;
            assert_eq!(get_node(c, "f9")?.unwrap().title.as_deref(), Some("Early"));
            Ok(())
        })
        .unwrap();
    }
}
