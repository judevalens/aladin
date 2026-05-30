-- +goose Up
-- +goose StatementBegin

-- Revert 00027 (canonical-as-log). That migration was an unauthorized
-- architecture change: it dropped the workspace_changes event log and bolted
-- updated_xid/is_deleted onto tree_nodes. The locked architecture is a generic,
-- durable outbox_events log (carries opaque event frames, replayable by xid) —
-- entity data stays in the canonical tables, which must NOT carry sync columns.
-- Plan: ~/.claude/plans/data-layer-offline-readable.md.
--
-- Forward-only: rather than mutate history, this migration cleanly undoes
-- 00027's schema. 00026's workspace_changes is recreated here so the DB matches
-- the (reset) code that still references it; the generic outbox_events table is
-- introduced by the next migration when the new server path is built.

-- Drop 00027's canonical-as-log additions on tree_nodes.
DROP INDEX IF EXISTS idx_tree_nodes_user_updated_xid;
ALTER TABLE tree_nodes DROP COLUMN IF EXISTS is_deleted;
ALTER TABLE tree_nodes DROP COLUMN IF EXISTS updated_xid;

-- Restore the plain (non-partial) unique sibling-position index.
DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;
CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''::text), "position");

-- Recreate the workspace_changes log 00027 had dropped (matches 00026), so the
-- schema is consistent with the reverted code. (It is itself superseded by the
-- generic outbox_events table in the next migration.)
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

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Re-apply 00027 (canonical-as-log) to go back down.
DROP TABLE IF EXISTS workspace_changes;
ALTER TABLE tree_nodes ADD COLUMN updated_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE tree_nodes ADD COLUMN is_deleted boolean NOT NULL DEFAULT false;
CREATE INDEX idx_tree_nodes_user_updated_xid ON tree_nodes (user_id, updated_xid);
DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;
CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''::text), "position")
    WHERE is_deleted = false;

-- +goose StatementEnd
