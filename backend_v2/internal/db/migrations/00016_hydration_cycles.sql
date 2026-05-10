-- +goose Up

ALTER TABLE sync_cycles
    ADD COLUMN IF NOT EXISTS last_hydrated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS target_kind TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_external_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_sync_cycles_active_target
    ON sync_cycles (source_id, kind, target_kind, target_external_id)
    WHERE status IN ('active', 'running') AND target_external_id <> '';

CREATE INDEX IF NOT EXISTS idx_sync_cycles_source_due
    ON sync_cycles (source_id, last_hydrated_at, created_at)
    WHERE status = 'active';

ALTER TABLE records
    ADD COLUMN IF NOT EXISTS source_revision BIGINT NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE records
    DROP COLUMN IF EXISTS source_revision;

DROP INDEX IF EXISTS idx_sync_cycles_source_due;
DROP INDEX IF EXISTS uq_sync_cycles_active_target;

ALTER TABLE sync_cycles
    DROP COLUMN IF EXISTS target_external_id,
    DROP COLUMN IF EXISTS target_kind,
    DROP COLUMN IF EXISTS last_hydrated_at;
