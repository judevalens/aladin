-- Shard local state — the per-shard key/value document store (design/SHARD_LOCAL_STATE.md).
-- One shard owns many small JSON documents addressed by stable path-shaped keys;
-- each key carries its own revision (granular optimistic concurrency) and a
-- tombstone (deleted_at) so realtime subscribers see deletions. `revision` doubles
-- as the sync-frame seq for the published channel (the only channel that syncs —
-- draft rows are the agent's server-side sandbox). Ownership lives on the artifact
-- row (shard_id -> artifacts.id, type='app'); enforced at the service layer, so no
-- user_id column here — viewer-scoped data uses key convention (viewer/<id>/...).

-- +goose Up
CREATE TABLE shard_kv (
    shard_id   text NOT NULL,
    channel    text NOT NULL,
    key        text NOT NULL,
    value      jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision   bigint NOT NULL DEFAULT 0,
    created_by uuid,
    updated_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    PRIMARY KEY (shard_id, channel, key)
);

CREATE INDEX shard_kv_prefix_idx
    ON shard_kv (shard_id, channel, key text_pattern_ops);

CREATE INDEX shard_kv_updated_idx
    ON shard_kv (shard_id, channel, updated_at DESC);

-- +goose Down
DROP TABLE shard_kv;
