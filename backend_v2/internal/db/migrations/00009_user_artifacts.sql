-- +goose Up

CREATE TABLE IF NOT EXISTS artifacts (
    id          TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    summary     TEXT,
    source_url  TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_artifacts_user_updated ON artifacts (user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifacts_type ON artifacts (type);

-- +goose Down

DROP INDEX IF EXISTS idx_artifacts_type;
DROP INDEX IF EXISTS idx_artifacts_user_updated;
DROP TABLE IF EXISTS artifacts;
