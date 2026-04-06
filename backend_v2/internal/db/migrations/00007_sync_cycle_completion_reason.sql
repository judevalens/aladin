-- +goose Up

ALTER TABLE sync_cycles
    ADD COLUMN IF NOT EXISTS completion_reason TEXT;

-- +goose Down

ALTER TABLE sync_cycles
    DROP COLUMN IF EXISTS completion_reason;
