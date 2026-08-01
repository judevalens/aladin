use rusqlite::Connection;

use super::DbResult;

const CURRENT_VERSION: i32 = 16;

/// Migrate the local cache schema. ATOMIC: every pending block AND the `user_version` stamp run in
/// ONE transaction, so a mid-migration failure rolls back cleanly — the version is never advanced
/// past what actually applied, and a re-run starts from the same version instead of failing forever
/// on a half-applied schema (which bricks the cache). PRAGMA user_version is transactional in SQLite.
///
/// `journal_mode`/`foreign_keys` are set by the caller on the bare connection BEFORE this (WAL can't
/// be changed inside a transaction).
pub fn migrate(conn: &mut Connection) -> DbResult<()> {
    let version: i32 = conn.query_row("PRAGMA user_version", [], |row| row.get(0))?;
    if version >= CURRENT_VERSION {
        return Ok(());
    }

    let tx = conn.transaction()?;
    if version < 1 {
        tx.execute_batch(MIGRATION_V1)?;
    }
    if version < 2 {
        tx.execute_batch(MIGRATION_V2)?;
    }
    if version < 3 {
        tx.execute_batch(MIGRATION_V3)?;
    }
    if version < 4 {
        tx.execute_batch(MIGRATION_V4)?;
    }
    if version < 5 {
        tx.execute_batch(MIGRATION_V5)?;
    }
    if version < 6 {
        tx.execute_batch(MIGRATION_V6)?;
    }
    if version < 7 {
        tx.execute_batch(MIGRATION_V7)?;
    }
    if version < 8 {
        tx.execute_batch(MIGRATION_V8)?;
    }
    if version < 9 {
        tx.execute_batch(MIGRATION_V9)?;
    }
    if version < 10 {
        tx.execute_batch(MIGRATION_V10)?;
    }
    if version < 11 {
        tx.execute_batch(MIGRATION_V11)?;
    }
    if version < 12 {
        tx.execute_batch(MIGRATION_V12)?;
    }
    if version < 13 {
        tx.execute_batch(MIGRATION_V13)?;
    }
    if version < 14 {
        tx.execute_batch(MIGRATION_V14)?;
    }
    if version < 15 {
        tx.execute_batch(MIGRATION_V15)?;
    }
    if version < 16 {
        tx.execute_batch(MIGRATION_V16)?;
    }
    tx.execute_batch(&format!("PRAGMA user_version = {CURRENT_VERSION};"))?;
    tx.commit()?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use rusqlite::Connection;

    // The atomic migrate applies the whole chain (fresh DB → CURRENT_VERSION with the latest
    // table present) and is a safe no-op on re-run.
    #[test]
    fn migrate_applies_full_chain_and_is_idempotent() {
        let mut conn = Connection::open_in_memory().unwrap();
        migrate(&mut conn).unwrap();

        let v: i32 = conn
            .query_row("PRAGMA user_version", [], |r| r.get(0))
            .unwrap();
        assert_eq!(v, CURRENT_VERSION, "version stamped to CURRENT_VERSION");

        // A table created by the newest migration (V14) exists — proves the chain ran, not just
        // the version stamp.
        let n: i32 = conn
            .query_row(
                "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='watchlists'",
                [],
                |r| r.get(0),
            )
            .unwrap();
        assert_eq!(n, 1, "latest migration's table present");

        // Re-run: early-returns, no error, no double-apply.
        migrate(&mut conn).unwrap();
    }
}

// Research bench (RESEARCH_SURFACE_PRD §5): a research folder is a third tree kind on
// the SAME nodes table, so it needs no table of its own — only somewhere to keep the
// extension's light fields (run state, exec mode, source kind) that ride its frame.
// Stored as the raw JSON object the frame carried; the read layer hands it to the UI.
const MIGRATION_V16: &str = "
ALTER TABLE nodes ADD COLUMN research_json TEXT;
";

// Claim layer removed — the "signal" kind is gone from the sync spine, so its local read
// cache goes with it. V13 is kept as history (never edit an applied migration); a fresh
// cache creates the table and immediately drops it here, which is harmless.
const MIGRATION_V15: &str = "
DROP TABLE IF EXISTS signals;
";

// Data-layer: watchlists as a synced entity ("watchlist" kind). Aggregate model — ONE row per
// list, its members carried in items_json ([{instrumentId,symbol,name,position}]); an add/remove
// re-emits the whole list. seq + is_deleted mirror the nodes guard. A pure read cache fed
// by frames; the server is the only writer.
const MIGRATION_V14: &str = "
CREATE TABLE IF NOT EXISTS watchlists (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT 'manual',
    position   INTEGER NOT NULL DEFAULT 0,
    items_json TEXT,
    seq        INTEGER NOT NULL DEFAULT 0,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0
);
";

// Data-layer: claims as a synced entity ("signal" kind). HISTORICAL — the claim layer was
// removed and V15 drops this table. Kept so the migration chain still replays in order.
const MIGRATION_V13: &str = "
CREATE TABLE IF NOT EXISTS signals (
    id            TEXT PRIMARY KEY,
    text          TEXT NOT NULL,
    polarity      TEXT,
    trust_tier    TEXT,
    subjects_json TEXT,
    assert_count  INTEGER NOT NULL DEFAULT 0,
    deny_count    INTEGER NOT NULL DEFAULT 0,
    supports      INTEGER NOT NULL DEFAULT 0,
    contradicts   INTEGER NOT NULL DEFAULT 0,
    qualifies     INTEGER NOT NULL DEFAULT 0,
    signal_score  REAL NOT NULL DEFAULT 0,
    seq           INTEGER NOT NULL DEFAULT 0,
    is_deleted    INTEGER NOT NULL DEFAULT 0,
    updated_at    INTEGER NOT NULL DEFAULT 0
);
";

// Data-layer redesign, cutover — drop the legacy local tables. The tree is now
// the unified `nodes` model (V10) + the sync control plane (V9 field_seq/
// field_intent/existence_intent) + sync_state; page CONTENT lives in
// Yjs/Hocuspocus. `page_metadata` and `backend_events` are retained for now.
const MIGRATION_V11: &str = "
DROP TABLE IF EXISTS page_content;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS outbox_mutations;
";

const MIGRATION_V1: &str = "
CREATE TABLE IF NOT EXISTS folders (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    title TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS folders_parent ON folders(parent_id);

CREATE TABLE IF NOT EXISTS artifacts (
    id TEXT PRIMARY KEY,
    folder_id TEXT,
    title TEXT NOT NULL,
    kind TEXT NOT NULL,
    content TEXT,
    source_url TEXT,
    resource_url TEXT,
    metadata_json TEXT,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS artifacts_folder ON artifacts(folder_id);

CREATE TABLE IF NOT EXISTS page_metadata (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    revision INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
";

const MIGRATION_V2: &str = "
ALTER TABLE folders ADD COLUMN sync_status TEXT NOT NULL DEFAULT 'SYNCED';
ALTER TABLE folders ADD COLUMN version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE artifacts ADD COLUMN sync_status TEXT NOT NULL DEFAULT 'SYNCED';
ALTER TABLE artifacts ADD COLUMN version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE page_metadata ADD COLUMN sync_status TEXT NOT NULL DEFAULT 'SYNCED';
ALTER TABLE page_metadata ADD COLUMN version INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS outbox_mutations (
    id TEXT PRIMARY KEY,
    entity_kind TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    op_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    mutation_id TEXT NOT NULL UNIQUE,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS outbox_mutations_created_at ON outbox_mutations(created_at);
CREATE INDEX IF NOT EXISTS outbox_mutations_entity ON outbox_mutations(entity_kind, entity_id);
";

const MIGRATION_V3: &str = "
DROP TABLE IF EXISTS browser_tree_cache;
";

const MIGRATION_V4: &str = "
ALTER TABLE folders ADD COLUMN kind TEXT NOT NULL DEFAULT 'folder';
ALTER TABLE folders ADD COLUMN artifact_id TEXT;
CREATE INDEX IF NOT EXISTS folders_kind ON folders(kind);
CREATE INDEX IF NOT EXISTS folders_artifact_id ON folders(artifact_id);
";

const MIGRATION_V5: &str = "
ALTER TABLE outbox_mutations ADD COLUMN status TEXT NOT NULL DEFAULT 'PENDING';
ALTER TABLE outbox_mutations ADD COLUMN last_error TEXT;
ALTER TABLE outbox_mutations ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
UPDATE outbox_mutations SET updated_at = created_at WHERE updated_at = 0;
CREATE INDEX IF NOT EXISTS outbox_mutations_status_attempts ON outbox_mutations(status, attempts, created_at);
";

const MIGRATION_V6: &str = "
ALTER TABLE folders ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
ALTER TABLE artifacts ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS folders_is_deleted ON folders(is_deleted);
CREATE INDEX IF NOT EXISTS artifacts_is_deleted ON artifacts(is_deleted);
";

const MIGRATION_V7: &str = "
CREATE TABLE IF NOT EXISTS backend_events (
    id TEXT PRIMARY KEY,
    event_kind TEXT NOT NULL,
    subscription_key_json TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL,
    validation_error TEXT,
    occurred_at TEXT NOT NULL,
    received_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS backend_events_status_updated_at ON backend_events(status, updated_at);
CREATE INDEX IF NOT EXISTS backend_events_kind_updated_at ON backend_events(event_kind, updated_at);
";

// M7.1: page_content table — mirrors backend page_documents (blocks JSON +
// revision). One row per page artifact; cascades on artifact delete.
const MIGRATION_V8: &str = "
CREATE TABLE IF NOT EXISTS page_content (
    id TEXT PRIMARY KEY REFERENCES artifacts(id) ON DELETE CASCADE,
    blocks TEXT NOT NULL DEFAULT '[]',
    revision INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    sync_status TEXT NOT NULL DEFAULT 'SYNCED',
    version INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS page_content_sync_status ON page_content(sync_status);
";

// Data-layer redesign, Phase 0 (client control plane). Plan:
// ~/.claude/plans/data-layer-sync-model.md. Additive — the legacy
// outbox_mutations/backend_events + folders/artifacts sync_status/version stay
// until cutover. Confirmed field VALUES keep living in the existing typed
// tables (folders/artifacts/...); these tables add only the sync control plane:
//   - field_seq:        per-(entity,field) newest-wins comparator (the seq guard)
//   - field_intent:     field-level offline intents (value read from the typed
//                       table at send; coalesce = set-membership per key)
//   - existence_intent: create/delete intents (linear lifecycle, terminal delete)
// client_id, the reconnect cursor, and next_local_seq live in sync_state (KV),
// seeded by Rust at startup.
const MIGRATION_V9: &str = "
CREATE TABLE IF NOT EXISTS field_seq (
    entity_kind TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    field       TEXT NOT NULL,
    seq         INTEGER NOT NULL,
    PRIMARY KEY (entity_kind, entity_id, field)
);

CREATE TABLE IF NOT EXISTS field_intent (
    entity_kind     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    field           TEXT NOT NULL,
    dirty_version   INTEGER NOT NULL,
    state           TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING | IN_FLIGHT | FAILED_RETRYABLE
    client_write_id TEXT,                              -- minted per dirty_version at first send
    local_seq       INTEGER NOT NULL,                  -- monotonic enqueue order (no timestamps)
    updated_at      INTEGER NOT NULL,
    PRIMARY KEY (entity_kind, entity_id, field)
);
CREATE INDEX IF NOT EXISTS field_intent_state ON field_intent(state, local_seq);

CREATE TABLE IF NOT EXISTS existence_intent (
    entity_kind     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    kind            TEXT NOT NULL,                      -- CREATE | DELETE
    intent_version  INTEGER NOT NULL,
    state           TEXT NOT NULL DEFAULT 'PENDING',
    client_write_id TEXT,
    local_seq       INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    PRIMARY KEY (entity_kind, entity_id)
);
CREATE INDEX IF NOT EXISTS existence_intent_state ON existence_intent(state, local_seq);
";

// Data-layer R1-C — the client read cache for the generic outbox.
// `nodes` becomes a pure read cache fed by sync FRAMES: per-ENTITY `seq` (the
// staleness guard; client applies iff incoming.seq > stored.seq) + `is_deleted`
// (soft-delete tombstone — the row is KEPT so a stale lower-seq upsert can't
// resurrect it; reads filter is_deleted=0). The client no longer writes locally
// or converges: the per-field intent log (field_seq/field_intent/
// existence_intent, V9) is removed — the server is the only writer, writes proxy
// through the Rust host to Go, and the change returns as a frame. The cursor in
// sync_state is now the uint64 xid (stored as a decimal string).
const MIGRATION_V12: &str = "
ALTER TABLE nodes ADD COLUMN seq INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS nodes_is_deleted ON nodes(is_deleted);
DROP TABLE IF EXISTS field_seq;
DROP TABLE IF EXISTS field_intent;
DROP TABLE IF EXISTS existence_intent;
";

// Data-layer redesign, Phase A/D — the clean local model. One node tree
// (folders + artifacts) with NO title/placement duplication, mapping 1:1 to the
// sync feed: entity_kind ∈ {folder, artifact}; per-field changes apply to the
// columns here (title, parentId→parent_id, position, type→artifact_type,
// content, sourceUrl→source_url, summary, metadata→metadata_json). `field_seq`
// (V9) is the per-field newest-wins guard; the cursor lives in sync_state.
//
// Additive for now (build stays green): the legacy folders/artifacts/
// page_metadata/page_content + outbox_mutations/backend_events tables are
// dropped at cutover, once the read path + apply move onto `nodes` and the old
// outbox/realtime engine is replaced.
const MIGRATION_V10: &str = "
CREATE TABLE IF NOT EXISTS nodes (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL,            -- 'folder' | 'artifact' | 'research'
    parent_id     TEXT,                     -- NULL = root
    position      INTEGER NOT NULL DEFAULT 0,
    title         TEXT,
    artifact_type TEXT,                     -- note | link | voice | file (NULL for folders)
    content       TEXT,
    source_url    TEXT,
    summary       TEXT,
    metadata_json TEXT,
    updated_at    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS nodes_parent ON nodes(parent_id);
CREATE INDEX IF NOT EXISTS nodes_kind ON nodes(kind);
";
