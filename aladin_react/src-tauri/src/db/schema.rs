use rusqlite::Connection;

use super::DbResult;

const CURRENT_VERSION: i32 = 6;

pub fn migrate(conn: &Connection) -> DbResult<()> {
    let version: i32 = conn.query_row("PRAGMA user_version", [], |row| row.get(0))?;
    if version < 1 {
        conn.execute_batch(MIGRATION_V1)?;
    }
    if version < 2 {
        conn.execute_batch(MIGRATION_V2)?;
    }
    if version < 3 {
        conn.execute_batch(MIGRATION_V3)?;
    }
    if version < 4 {
        conn.execute_batch(MIGRATION_V4)?;
    }
    if version < 5 {
        conn.execute_batch(MIGRATION_V5)?;
    }
    if version < 6 {
        conn.execute_batch(MIGRATION_V6)?;
    }
    conn.execute_batch(&format!("PRAGMA user_version = {CURRENT_VERSION};"))?;
    Ok(())
}

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
