-- +goose Up

CREATE TABLE IF NOT EXISTS folders (
    id          TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id   TEXT REFERENCES folders(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE artifacts
    ADD COLUMN IF NOT EXISTS folder_id TEXT REFERENCES folders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_folders_user_parent_title ON folders (user_id, parent_id, title);
CREATE INDEX IF NOT EXISTS idx_artifacts_user_folder_updated ON artifacts (user_id, folder_id, updated_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_artifacts_user_folder_updated;
DROP INDEX IF EXISTS idx_folders_user_parent_title;

ALTER TABLE artifacts
    DROP COLUMN IF EXISTS folder_id;

DROP TABLE IF EXISTS folders;
