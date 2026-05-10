-- +goose Up

ALTER TABLE artifacts
    DROP COLUMN IF EXISTS folder_id;

-- +goose Down

ALTER TABLE artifacts
    ADD COLUMN IF NOT EXISTS folder_id TEXT;
