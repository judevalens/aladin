-- +goose Up

ALTER TABLE sync_cycles
    DROP COLUMN IF EXISTS covered_until;

-- +goose Down

ALTER TABLE sync_cycles
    ADD COLUMN IF NOT EXISTS covered_until JSONB NOT NULL DEFAULT '{}';
