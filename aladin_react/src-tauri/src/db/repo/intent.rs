use rusqlite::{params, Connection, OptionalExtension};

use crate::db::DbResult;

// Data-layer redesign, Phase B (client) — the offline write intent log.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// A local mutation writes `nodes` optimistically and enqueues an INTENT (what
// to do, not a value snapshot — the value is read from `nodes` at send time, so
// typed-then-changed values never ship). The sender drains intents in
// local_seq order and pushes them to POST /api/sync/push.
//
// Idempotency / ordering: one monotonic counter (next_local_seq) feeds BOTH the
// drain order (local_seq) AND the mutationId number (clientWriteId =
// "clientId:localSeq"). Because a coalescing re-enqueue assigns a NEW counter,
// the re-dirtied intent sorts last and its mutationId number is strictly
// greater — so the server's per-client high-water dedup (apply iff
// counter > last_applied) stays correct under coalescing. clientWriteId is
// stable per (intent, version) across retries: a crashed/duplicate send is a
// server-side no-op, so it can't re-apply a stale value over a newer peer write
// (review F1).
//
// Versions (dirty_version / intent_version) give a compare-and-set on resolve:
// success deletes the intent only if its version is unchanged since send; a
// concurrent re-enqueue bumps the version, so the stale resolve no-ops and the
// newer intent re-sends. No separate IN_FLIGHT state is needed (a crashed send
// leaves the intent PENDING → re-sent → deduped server-side), and create→delete
// while unsent overwrites the existence row (version-CAS handles the in-flight
// race), so explicit annihilation is unnecessary in v1.

const CLIENT_ID_KEY: &str = "client_id";
const LOCAL_SEQ_KEY: &str = "next_local_seq";

pub const KIND_CREATE: &str = "CREATE";
pub const KIND_DELETE: &str = "DELETE";
const STATE_PENDING: &str = "PENDING";

/// What a pending intent represents, for the sender to build a mutation.
#[derive(Debug, Clone, PartialEq)]
pub enum IntentOp {
    /// Create the entity (the sender reads its initial fields from `nodes`).
    Create,
    /// Delete the entity (subtree).
    Delete,
    /// Set one field (the sender reads its current value from `nodes`).
    Field(String),
}

/// One enqueued intent, ready to send.
#[derive(Debug, Clone, PartialEq)]
pub struct PendingIntent {
    pub local_seq: i64,
    pub entity_kind: String,
    pub entity_id: String,
    pub op: IntentOp,
    /// "clientId:localSeq" — the mutationId for idempotency/dedup.
    pub client_write_id: String,
    /// dirty_version (field) or intent_version (existence) at enqueue, for the
    /// compare-and-set on resolve.
    pub version: i64,
}

fn get_state(conn: &Connection, key: &str) -> DbResult<Option<String>> {
    Ok(conn
        .query_row("SELECT value FROM sync_state WHERE key = ?1", params![key], |r| r.get(0))
        .optional()?)
}

fn set_state(conn: &Connection, key: &str, value: &str) -> DbResult<()> {
    conn.execute(
        "INSERT INTO sync_state (key, value) VALUES (?1, ?2)
         ON CONFLICT(key) DO UPDATE SET value = excluded.value",
        params![key, value],
    )?;
    Ok(())
}

/// Returns this install's stable client id, minting one (random 128-bit hex via
/// SQLite's randomblob) on first call.
pub fn client_id(conn: &Connection) -> DbResult<String> {
    if let Some(id) = get_state(conn, CLIENT_ID_KEY)? {
        return Ok(id);
    }
    let id: String = conn.query_row("SELECT lower(hex(randomblob(16)))", [], |r| r.get(0))?;
    set_state(conn, CLIENT_ID_KEY, &id)?;
    Ok(id)
}

/// Mints the next (seq, clientWriteId) for an enqueue/coalesce.
fn mint(conn: &Connection) -> DbResult<(i64, String)> {
    let cur: i64 = get_state(conn, LOCAL_SEQ_KEY)?
        .and_then(|s| s.parse().ok())
        .unwrap_or(0);
    let seq = cur + 1;
    set_state(conn, LOCAL_SEQ_KEY, &seq.to_string())?;
    let cwid = format!("{}:{}", client_id(conn)?, seq);
    Ok((seq, cwid))
}

fn now_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0)
}

/// Enqueues a create intent for a new entity.
pub fn enqueue_create(conn: &Connection, entity_kind: &str, entity_id: &str) -> DbResult<()> {
    upsert_existence(conn, entity_kind, entity_id, KIND_CREATE)
}

/// Enqueues a delete intent. Overwriting an unsent create with a delete is safe:
/// only the delete sends (server delete of a never-created entity is an
/// idempotent no-op tombstone); if the create was mid-send, the version-CAS on
/// its resolve no-ops and the delete still sends.
pub fn enqueue_delete(conn: &Connection, entity_kind: &str, entity_id: &str) -> DbResult<()> {
    // Field edits for a deleted entity are moot.
    conn.execute(
        "DELETE FROM field_intent WHERE entity_kind = ?1 AND entity_id = ?2",
        params![entity_kind, entity_id],
    )?;
    upsert_existence(conn, entity_kind, entity_id, KIND_DELETE)
}

fn upsert_existence(conn: &Connection, entity_kind: &str, entity_id: &str, kind: &str) -> DbResult<()> {
    let (seq, cwid) = mint(conn)?;
    let now = now_ms();
    conn.execute(
        "INSERT INTO existence_intent (entity_kind, entity_id, kind, intent_version, state, client_write_id, local_seq, updated_at)
         VALUES (?1, ?2, ?3, 1, ?4, ?5, ?6, ?7)
         ON CONFLICT(entity_kind, entity_id) DO UPDATE SET
            kind = excluded.kind,
            intent_version = existence_intent.intent_version + 1,
            state = excluded.state,
            client_write_id = excluded.client_write_id,
            local_seq = excluded.local_seq,
            updated_at = excluded.updated_at",
        params![entity_kind, entity_id, kind, STATE_PENDING, cwid, seq, now],
    )?;
    Ok(())
}

/// Enqueues (or coalesces) a field-set intent. Coalescing bumps dirty_version
/// and re-mints clientWriteId/local_seq, so the latest value ships once.
pub fn enqueue_field(conn: &Connection, entity_kind: &str, entity_id: &str, field: &str) -> DbResult<()> {
    let (seq, cwid) = mint(conn)?;
    let now = now_ms();
    conn.execute(
        "INSERT INTO field_intent (entity_kind, entity_id, field, dirty_version, state, client_write_id, local_seq, updated_at)
         VALUES (?1, ?2, ?3, 1, ?4, ?5, ?6, ?7)
         ON CONFLICT(entity_kind, entity_id, field) DO UPDATE SET
            dirty_version = field_intent.dirty_version + 1,
            state = excluded.state,
            client_write_id = excluded.client_write_id,
            local_seq = excluded.local_seq,
            updated_at = excluded.updated_at",
        params![entity_kind, entity_id, field, STATE_PENDING, cwid, seq, now],
    )?;
    Ok(())
}

/// All pending intents (existence + field), in drain order (local_seq asc).
pub fn list_pending(conn: &Connection) -> DbResult<Vec<PendingIntent>> {
    let mut out: Vec<PendingIntent> = Vec::new();

    let mut existence = conn.prepare(
        "SELECT entity_kind, entity_id, kind, intent_version, client_write_id, local_seq
           FROM existence_intent WHERE state = ?1",
    )?;
    let rows = existence.query_map(params![STATE_PENDING], |r| {
        let kind: String = r.get(2)?;
        Ok((
            r.get::<_, String>(0)?,
            r.get::<_, String>(1)?,
            kind,
            r.get::<_, i64>(3)?,
            r.get::<_, Option<String>>(4)?,
            r.get::<_, i64>(5)?,
        ))
    })?;
    for row in rows {
        let (entity_kind, entity_id, kind, version, cwid, local_seq) = row?;
        let Some(client_write_id) = cwid else { continue };
        out.push(PendingIntent {
            local_seq,
            entity_kind,
            entity_id,
            op: if kind == KIND_DELETE { IntentOp::Delete } else { IntentOp::Create },
            client_write_id,
            version,
        });
    }

    let mut fields = conn.prepare(
        "SELECT entity_kind, entity_id, field, dirty_version, client_write_id, local_seq
           FROM field_intent WHERE state = ?1",
    )?;
    let rows = fields.query_map(params![STATE_PENDING], |r| {
        Ok((
            r.get::<_, String>(0)?,
            r.get::<_, String>(1)?,
            r.get::<_, String>(2)?,
            r.get::<_, i64>(3)?,
            r.get::<_, Option<String>>(4)?,
            r.get::<_, i64>(5)?,
        ))
    })?;
    for row in rows {
        let (entity_kind, entity_id, field, version, cwid, local_seq) = row?;
        let Some(client_write_id) = cwid else { continue };
        out.push(PendingIntent {
            local_seq,
            entity_kind,
            entity_id,
            op: IntentOp::Field(field),
            client_write_id,
            version,
        });
    }

    out.sort_by_key(|i| i.local_seq);
    Ok(out)
}

/// Clears an existence intent on a successful/duplicate ack, iff its version is
/// unchanged since send (compare-and-set). Returns true if cleared.
pub fn resolve_existence(conn: &Connection, entity_kind: &str, entity_id: &str, sent_version: i64) -> DbResult<bool> {
    let n = conn.execute(
        "DELETE FROM existence_intent WHERE entity_kind = ?1 AND entity_id = ?2 AND intent_version = ?3",
        params![entity_kind, entity_id, sent_version],
    )?;
    Ok(n > 0)
}

/// Clears a field intent on a successful/duplicate ack, iff unchanged (CAS).
pub fn resolve_field(conn: &Connection, entity_kind: &str, entity_id: &str, field: &str, sent_version: i64) -> DbResult<bool> {
    let n = conn.execute(
        "DELETE FROM field_intent WHERE entity_kind = ?1 AND entity_id = ?2 AND field = ?3 AND dirty_version = ?4",
        params![entity_kind, entity_id, field, sent_version],
    )?;
    Ok(n > 0)
}

/// Drops an existence intent unconditionally (server rejected it — reconcile
/// via pull heals the optimistic local state).
pub fn drop_existence(conn: &Connection, entity_kind: &str, entity_id: &str) -> DbResult<()> {
    conn.execute(
        "DELETE FROM existence_intent WHERE entity_kind = ?1 AND entity_id = ?2",
        params![entity_kind, entity_id],
    )?;
    Ok(())
}

/// Drops a field intent unconditionally (server rejected it).
pub fn drop_field(conn: &Connection, entity_kind: &str, entity_id: &str, field: &str) -> DbResult<()> {
    conn.execute(
        "DELETE FROM field_intent WHERE entity_kind = ?1 AND entity_id = ?2 AND field = ?3",
        params![entity_kind, entity_id, field],
    )?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::db::Db;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn test_db(name: &str) -> Db {
        let nanos = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos();
        let path = std::env::temp_dir().join(format!(
            "aladin_intent_test_{name}_{}_{}.sqlite",
            std::process::id(),
            nanos
        ));
        Db::open(PathBuf::from(path)).unwrap()
    }

    #[test]
    fn client_id_is_stable() {
        let db = test_db("client_id");
        db.with_conn(|c| {
            let a = client_id(c)?;
            let b = client_id(c)?;
            assert_eq!(a, b);
            assert_eq!(a.len(), 32); // 16 bytes hex
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn create_then_fields_drain_in_order() {
        let db = test_db("drain_order");
        db.with_conn(|c| {
            enqueue_create(c, "folder", "f1")?;
            enqueue_field(c, "folder", "f1", "title")?;
            let pending = list_pending(c)?;
            assert_eq!(pending.len(), 2);
            assert_eq!(pending[0].op, IntentOp::Create);
            assert_eq!(pending[1].op, IntentOp::Field("title".into()));
            // clientWriteId = clientId:localSeq, strictly increasing in drain order.
            assert!(pending[0].local_seq < pending[1].local_seq);
            assert!(pending[0].client_write_id.ends_with(&format!(":{}", pending[0].local_seq)));
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn field_coalesces_and_re_sorts_last() {
        let db = test_db("coalesce");
        db.with_conn(|c| {
            enqueue_field(c, "folder", "f1", "title")?;
            enqueue_field(c, "artifact", "a1", "title")?;
            // Re-edit f1.title → coalesces (one row) + sorts after a1 with a higher seq.
            enqueue_field(c, "folder", "f1", "title")?;
            let pending = list_pending(c)?;
            assert_eq!(pending.len(), 2);
            assert_eq!(pending[0].entity_id, "a1");
            assert_eq!(pending[1].entity_id, "f1");
            // f1's dirty_version bumped to 2.
            assert_eq!(pending[1].version, 2);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn delete_overwrites_create_and_drops_fields() {
        let db = test_db("delete_overwrites");
        db.with_conn(|c| {
            enqueue_create(c, "artifact", "a1")?;
            enqueue_field(c, "artifact", "a1", "title")?;
            enqueue_delete(c, "artifact", "a1")?;
            let pending = list_pending(c)?;
            // Only the delete remains.
            assert_eq!(pending.len(), 1);
            assert_eq!(pending[0].op, IntentOp::Delete);
            Ok(())
        })
        .unwrap();
    }

    #[test]
    fn resolve_is_compare_and_set() {
        let db = test_db("resolve_cas");
        db.with_conn(|c| {
            enqueue_field(c, "folder", "f1", "title")?;
            let sent = list_pending(c)?[0].clone();
            // A concurrent re-edit bumps dirty_version while the send is in flight.
            enqueue_field(c, "folder", "f1", "title")?;
            // The stale resolve (old version) must NOT clear it.
            assert!(!resolve_field(c, "folder", "f1", "title", sent.version)?);
            assert_eq!(list_pending(c)?.len(), 1);
            // Resolving the current version clears it.
            let current = list_pending(c)?[0].clone();
            assert!(resolve_field(c, "folder", "f1", "title", current.version)?);
            assert!(list_pending(c)?.is_empty());
            Ok(())
        })
        .unwrap();
    }
}
