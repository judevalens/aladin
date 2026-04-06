-- +goose Up

ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS last_picked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_refresh_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sources_scheduler
    ON sources (last_picked_at, last_synced_at, created_at)
    WHERE sync_mode = 'poll' AND sync_state = 'active' AND sync_status = 'idle';

-- +goose Down

DROP INDEX IF EXISTS idx_sources_scheduler;

ALTER TABLE sources
    DROP COLUMN IF EXISTS last_refresh_at,
    DROP COLUMN IF EXISTS last_picked_at;
