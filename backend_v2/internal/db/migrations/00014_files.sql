-- +goose Up

CREATE TABLE IF NOT EXISTS files (
    id          TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_files_user_uploaded ON files (user_id, uploaded_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_files_user_uploaded;
DROP TABLE IF EXISTS files;
