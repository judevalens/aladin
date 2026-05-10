-- +goose Up

CREATE TABLE IF NOT EXISTS tree_nodes (
    id          TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id   TEXT REFERENCES tree_nodes(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('folder', 'artifact')),
    title       TEXT,
    artifact_id TEXT REFERENCES artifacts(id) ON DELETE CASCADE,
    position    BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (kind = 'folder' AND artifact_id IS NULL AND title IS NOT NULL)
        OR
        (kind = 'artifact' AND artifact_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tree_nodes_user_parent_position
    ON tree_nodes (user_id, COALESCE(parent_id, ''), position);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tree_nodes_user_artifact
    ON tree_nodes (user_id, artifact_id)
    WHERE artifact_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tree_nodes_user_parent_kind
    ON tree_nodes (user_id, parent_id, kind, position);
-- +goose Down

DROP INDEX IF EXISTS idx_tree_nodes_user_parent_kind;
DROP INDEX IF EXISTS idx_tree_nodes_user_artifact;
DROP INDEX IF EXISTS idx_tree_nodes_user_parent_position;

DROP TABLE IF EXISTS tree_nodes;