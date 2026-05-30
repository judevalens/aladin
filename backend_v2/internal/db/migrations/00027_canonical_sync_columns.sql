-- +goose Up

-- Data-layer redesign R1 — canonical-as-log (server-authoritative cache).
-- Plan: ~/.claude/plans/data-layer-offline-readable.md.
--
-- No separate event outbox. The canonical tables ARE the log: each synced
-- entity carries `updated_xid` (the committing transaction's xid8) + a soft
-- `is_deleted` flag. The client pulls `WHERE updated_xid > cursor AND
-- updated_xid < pg_snapshot_xmin(...)` — gap-free by construction (commit
-- visibility), repo-agnostic, one ordered scan. Delete events stay visible to a
-- catching-up client because the row is soft-deleted (tombstone), not removed.
--
-- `tree_nodes` is the per-entity SPINE: every entity (folder AND artifact) has
-- exactly one tree_nodes row, so the sync layer sees one entity = one log row.
-- An artifact's content lives in `artifacts`; the SERVICE layer (ArtifactService)
-- is responsible for touching the owning node's updated_xid when artifact
-- metadata changes — that relationship is business logic, not a DB trigger.
--
-- `updated_xid` is stamped EXPLICITLY by the repo on every tree_nodes write
-- (`SET updated_xid = pg_current_xact_id()`), not by a trigger. The DEFAULT
-- below only backfills existing rows / guards inserts that forget.

-- Drop the old field-level feed (superseded; the canonical tables are the log).
DROP TABLE IF EXISTS workspace_changes;

ALTER TABLE tree_nodes
    ADD COLUMN updated_xid xid8    NOT NULL DEFAULT pg_current_xact_id(),
    ADD COLUMN is_deleted  boolean NOT NULL DEFAULT false;

-- Pull read pattern: WHERE user_id = $1 AND updated_xid > $cursor
-- AND updated_xid < $horizon ORDER BY updated_xid.
CREATE INDEX idx_tree_nodes_user_updated_xid ON tree_nodes (user_id, updated_xid);

-- A soft-deleted node must not squat a sibling (user, parent, position) slot, so
-- uniqueness applies to LIVE rows only. Replace the index with a partial one.
DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;
CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''::text), "position")
    WHERE is_deleted = false;

-- +goose Down

DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;
CREATE UNIQUE INDEX idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''::text), "position");

DROP INDEX IF EXISTS idx_tree_nodes_user_updated_xid;
ALTER TABLE tree_nodes DROP COLUMN IF EXISTS is_deleted;
ALTER TABLE tree_nodes DROP COLUMN IF EXISTS updated_xid;

-- (workspace_changes is not recreated on down — it was R0 scaffolding.)
