-- +goose Up

ALTER TABLE sync_cycles
    ADD COLUMN IF NOT EXISTS head_boundary JSONB NOT NULL DEFAULT '{}';

-- +goose Down

ALTER TABLE sync_cycles
    DROP COLUMN IF EXISTS head_boundary;
