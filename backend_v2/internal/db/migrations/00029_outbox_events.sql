-- +goose Up
-- +goose StatementBegin

-- Data-layer R1 — the generic, durable outbox log (server-authoritative sync).
-- Architecture: ~/.claude/plans/data-layer-offline-readable.md.
--
-- outbox_events is ONE generic append-only log: simultaneously the WS delivery
-- source AND the client replay log, ordered by `xid` (the committing txn's
-- xid8). Entity DATA stays in the canonical tables (tree_nodes/artifacts); the
-- outbox carries opaque event frames (payload JSONB). A write txn appends ONE
-- 'data_event' row in the SAME commit as the canonical write (transactional
-- outbox -> durable + atomic). Clients pull `WHERE xid > cursor AND xid <
-- pg_snapshot_xmin(...)` (gap-free) and replay; `xid` is the resume cursor.
--
-- This SUPERSEDES the field-level workspace_changes feed (00026) and the
-- client-push idempotency ledger sync_clients (the client write path is removed;
-- the server is the only writer).

CREATE TABLE outbox_events (
    xid        xid8        NOT NULL DEFAULT pg_current_xact_id(), -- order + cursor + replay key
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT        NOT NULL,   -- 'data_event' (room for 'notification','job',... later)
    payload    JSONB       NOT NULL,   -- OPAQUE to the engine; a data_event carries a FRAME
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Pull/resume read pattern: WHERE user_id = $1 AND xid >= $cursor AND xid < $horizon
-- ORDER BY xid. (No PK on xid alone -- one txn appends one row, but xid is the
-- commit-order key, not a uniqueness constraint.)
CREATE INDEX idx_outbox_user_xid ON outbox_events (user_id, xid);

-- tree_nodes is the per-entity SPINE: every entity (folder AND artifact) has
-- exactly one tree_nodes row, and for artifacts tree_nodes.id == artifact_id, so
-- one row carries the sync state for one logical entity.
--   seq:        per-entity version (the staleness guard). Server bumps on every
--               write to that entity; client applies iff incoming.seq > stored.
--   is_deleted: soft-delete tombstone. Delete keeps the row (is_deleted=true,
--               seq bumped) so a stale lower-seq upsert can't resurrect it;
--               reads filter is_deleted=false. (Artifacts inherit their entity's
--               tombstone via the tree_nodes join -- no column needed there.)
ALTER TABLE tree_nodes
    ADD COLUMN seq        BIGINT  NOT NULL DEFAULT 0,
    ADD COLUMN is_deleted BOOLEAN NOT NULL DEFAULT false;

-- A soft-deleted node retains its (parent, position) slot, so sibling-position
-- uniqueness must apply to LIVE rows only. Replace the plain index with a
-- partial one.
DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;
CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''::text), "position")
    WHERE is_deleted = false;

-- Retire the superseded field-level feed + the client-push idempotency ledger.
DROP TABLE IF EXISTS workspace_changes;
DROP TABLE IF EXISTS sync_clients;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restore the plain (non-partial) unique index, drop the sync columns.
DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;
CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''::text), "position");
ALTER TABLE tree_nodes DROP COLUMN IF EXISTS is_deleted;
ALTER TABLE tree_nodes DROP COLUMN IF EXISTS seq;

DROP TABLE IF EXISTS outbox_events;

-- Recreate the workspace_changes feed (matches 00026/00028) so down restores the
-- prior schema the reverted code expects.
CREATE TABLE IF NOT EXISTS workspace_changes (
    seq         BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_kind TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    op          TEXT NOT NULL CHECK (op IN ('create', 'update', 'delete')),
    field       TEXT,
    value       JSONB,
    mutation_id TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (op = 'update' AND field IS NOT NULL)
        OR (op IN ('create', 'delete') AND field IS NULL)
    )
);
CREATE INDEX IF NOT EXISTS idx_workspace_changes_user_seq ON workspace_changes (user_id, seq);

CREATE TABLE IF NOT EXISTS sync_clients (
    client_id    TEXT PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_applied BIGINT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sync_clients_user ON sync_clients (user_id);

-- +goose StatementEnd
