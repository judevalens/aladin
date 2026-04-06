-- +goose Up

CREATE TABLE IF NOT EXISTS sync_cycles (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id      UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    cursor         JSONB NOT NULL DEFAULT '{}',
    covered_until  JSONB NOT NULL DEFAULT '{}',
    last_picked_at TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sync_cycles_source_active
    ON sync_cycles (source_id, created_at)
    WHERE status IN ('active', 'running');

-- +goose Down

DROP INDEX IF EXISTS idx_sync_cycles_source_active;
DROP TABLE IF EXISTS sync_cycles;
