-- +goose Up

ALTER TABLE records
    DROP CONSTRAINT IF EXISTS records_snapshot_id_fkey;

ALTER TABLE records
    DROP CONSTRAINT IF EXISTS records_source_id_fkey;

DROP INDEX IF EXISTS uq_records_source_external;
DROP INDEX IF EXISTS idx_records_top_level;
DROP INDEX IF EXISTS idx_records_external;
DROP INDEX IF EXISTS idx_records_source;
DROP INDEX IF EXISTS idx_records_snapshot;

ALTER TABLE records
    DROP COLUMN IF EXISTS snapshot_id,
    DROP COLUMN IF EXISTS source_id;

DELETE FROM tenant_item_matches tim
USING source_subscriptions ss
WHERE tim.subscription_id = ss.id
  AND ss.policy ? 'legacy_source_id';

DELETE FROM source_subscriptions
WHERE policy ? 'legacy_source_id';

DELETE FROM provider_streams ps
USING sources s
WHERE ps.id = s.id;

DROP INDEX IF EXISTS idx_snapshots_source;
ALTER TABLE snapshots
    DROP CONSTRAINT IF EXISTS snapshots_source_id_fkey;
DROP TABLE IF EXISTS snapshots;

DROP INDEX IF EXISTS idx_sync_cycles_source_due;
DROP INDEX IF EXISTS uq_sync_cycles_active_target;
DROP INDEX IF EXISTS idx_sync_cycles_source_active;

ALTER TABLE sync_cycles
    DROP CONSTRAINT IF EXISTS sync_cycles_source_id_fkey;

ALTER TABLE sync_cycles
    DROP COLUMN IF EXISTS source_id;

DROP TABLE IF EXISTS sync_jobs;

DROP INDEX IF EXISTS idx_sources_due;
DROP INDEX IF EXISTS idx_sources_claimable;
DROP INDEX IF EXISTS idx_sources_scheduler;
DROP TABLE IF EXISTS sources CASCADE;

-- +goose Down

CREATE TABLE IF NOT EXISTS sources (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID NOT NULL REFERENCES users(id),
    kg_id                   UUID NOT NULL REFERENCES knowledge_graphs(id),
    name                    TEXT NOT NULL,
    type                    TEXT NOT NULL,
    sync_mode               TEXT NOT NULL,
    sync_state              TEXT NOT NULL DEFAULT 'pending',
    sync_status             TEXT NOT NULL DEFAULT 'idle',
    config                  JSONB NOT NULL DEFAULT '{}',
    auto_promote_threshold  FLOAT NOT NULL DEFAULT 0.85,
    suggest_threshold       FLOAT NOT NULL DEFAULT 0.60,
    next_sync_at            TIMESTAMPTZ,
    last_synced_at          TIMESTAMPTZ,
    last_sync_started_at    TIMESTAMPTZ,
    last_picked_at          TIMESTAMPTZ,
    last_refresh_at         TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id       UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'processing',
    expected_jobs   INTEGER NOT NULL DEFAULT 0,
    completed_jobs  INTEGER NOT NULL DEFAULT 0,
    record_count    INTEGER NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_id, version)
);

ALTER TABLE sync_cycles
    ADD COLUMN IF NOT EXISTS source_id UUID REFERENCES sources(id) ON DELETE CASCADE;

ALTER TABLE records
    ADD COLUMN IF NOT EXISTS source_id UUID REFERENCES sources(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS snapshot_id UUID REFERENCES snapshots(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sources_due
    ON sources (next_sync_at)
    WHERE sync_mode = 'poll' AND sync_state = 'active';

CREATE INDEX IF NOT EXISTS idx_sources_claimable
    ON sources (last_synced_at)
    WHERE sync_mode = 'poll' AND sync_state = 'active' AND sync_status = 'idle';

CREATE INDEX IF NOT EXISTS idx_sources_scheduler
    ON sources (last_picked_at, last_synced_at, created_at)
    WHERE sync_mode = 'poll' AND sync_state = 'active' AND sync_status = 'idle';

CREATE INDEX IF NOT EXISTS idx_snapshots_source
    ON snapshots (source_id);

CREATE INDEX IF NOT EXISTS idx_records_source
    ON records (source_id);

CREATE INDEX IF NOT EXISTS idx_records_snapshot
    ON records (snapshot_id);

CREATE INDEX IF NOT EXISTS idx_records_external
    ON records (source_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_records_top_level
    ON records (source_id, created_at)
    WHERE parent_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_records_source_external
    ON records (source_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_sync_cycles_source_active
    ON sync_cycles (source_id, created_at)
    WHERE status IN ('active', 'running');
