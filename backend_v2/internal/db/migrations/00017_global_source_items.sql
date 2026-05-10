-- +goose Up

CREATE TABLE IF NOT EXISTS provider_streams (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider                TEXT NOT NULL,
    stream_kind             TEXT NOT NULL,
    stream_key              TEXT NOT NULL,
    name                    TEXT NOT NULL,
    visibility              TEXT NOT NULL DEFAULT 'public',
    sync_mode               TEXT NOT NULL DEFAULT 'poll',
    sync_state              TEXT NOT NULL DEFAULT 'active',
    sync_status             TEXT NOT NULL DEFAULT 'idle',
    config                  JSONB NOT NULL DEFAULT '{}',
    last_picked_at          TIMESTAMPTZ,
    last_refresh_at         TIMESTAMPTZ,
    last_sync_started_at    TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, stream_kind, stream_key)
);

CREATE TABLE IF NOT EXISTS source_subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kg_id               UUID NOT NULL REFERENCES knowledge_graphs(id) ON DELETE CASCADE,
    provider_stream_id  UUID NOT NULL REFERENCES provider_streams(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    visibility          TEXT NOT NULL DEFAULT 'public',
    policy              JSONB NOT NULL DEFAULT '{}',
    status              TEXT NOT NULL DEFAULT 'active',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, kg_id, provider_stream_id)
);

CREATE TABLE IF NOT EXISTS source_items (
    id                  TEXT PRIMARY KEY,
    provider_stream_id  UUID NOT NULL REFERENCES provider_streams(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL,
    external_id         TEXT NOT NULL,
    owner_user_id       UUID REFERENCES users(id) ON DELETE CASCADE,
    item_type           TEXT NOT NULL,
    title               TEXT NOT NULL,
    canonical_url       TEXT,
    source_url          TEXT,
    content_excerpt     TEXT NOT NULL DEFAULT '',
    context_excerpt     TEXT NOT NULL DEFAULT '',
    source_revision     BIGINT NOT NULL DEFAULT 0,
    provider_metadata   JSONB NOT NULL DEFAULT '{}',
    hydration_metadata  JSONB NOT NULL DEFAULT '{}',
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    hydrated_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_source_items_public
    ON source_items (provider, external_id)
    WHERE owner_user_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_source_items_private
    ON source_items (provider, external_id, owner_user_id)
    WHERE owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_source_items_stream_revision
    ON source_items (provider_stream_id, source_revision DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS source_item_enrichments (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_item_id          TEXT NOT NULL REFERENCES source_items(id) ON DELETE CASCADE,
    source_revision         BIGINT NOT NULL,
    summary                 TEXT NOT NULL DEFAULT '',
    entities                JSONB NOT NULL DEFAULT '[]',
    topics                  JSONB NOT NULL DEFAULT '[]',
    key_claims              JSONB NOT NULL DEFAULT '[]',
    low_confidence_entities JSONB NOT NULL DEFAULT '[]',
    embedding               vector(1536),
    status                  TEXT NOT NULL DEFAULT 'ready',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_item_id, source_revision)
);

CREATE TABLE IF NOT EXISTS tenant_item_matches (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id     UUID NOT NULL REFERENCES source_subscriptions(id) ON DELETE CASCADE,
    source_item_id      TEXT NOT NULL REFERENCES source_items(id) ON DELETE CASCADE,
    source_revision     BIGINT NOT NULL,
    match_source        TEXT NOT NULL,
    overlap_entities    JSONB NOT NULL DEFAULT '[]',
    relevance_status    TEXT NOT NULL DEFAULT 'pending',
    relevance_score     FLOAT,
    relevance_reason    TEXT,
    record_id           TEXT REFERENCES records(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (subscription_id, source_item_id, source_revision)
);

CREATE INDEX IF NOT EXISTS idx_source_subscriptions_stream_active
    ON source_subscriptions (provider_stream_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_tenant_item_matches_subscription
    ON tenant_item_matches (subscription_id, created_at DESC);

ALTER TABLE sync_cycles
    ADD COLUMN IF NOT EXISTS provider_stream_id UUID REFERENCES provider_streams(id) ON DELETE CASCADE;

ALTER TABLE sync_cycles
    ALTER COLUMN source_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_sync_cycles_provider_stream_active
    ON sync_cycles (provider_stream_id, created_at)
    WHERE status IN ('active', 'running');

CREATE UNIQUE INDEX IF NOT EXISTS uq_sync_cycles_provider_stream_active_target
    ON sync_cycles (provider_stream_id, kind, target_kind, target_external_id)
    WHERE status IN ('active', 'running')
      AND target_external_id IS NOT NULL;

ALTER TABLE records
    DROP CONSTRAINT IF EXISTS records_source_id_fkey;

-- +goose Down

DROP INDEX IF EXISTS uq_sync_cycles_provider_stream_active_target;
DROP INDEX IF EXISTS idx_sync_cycles_provider_stream_active;

ALTER TABLE sync_cycles
    ALTER COLUMN source_id SET NOT NULL;

ALTER TABLE sync_cycles
    DROP COLUMN IF EXISTS provider_stream_id;

DROP INDEX IF EXISTS idx_tenant_item_matches_subscription;
DROP INDEX IF EXISTS idx_source_subscriptions_stream_active;
DROP TABLE IF EXISTS tenant_item_matches;
DROP TABLE IF EXISTS source_item_enrichments;
DROP INDEX IF EXISTS idx_source_items_stream_revision;
DROP INDEX IF EXISTS uq_source_items_private;
DROP INDEX IF EXISTS uq_source_items_public;
DROP TABLE IF EXISTS source_items;
DROP TABLE IF EXISTS source_subscriptions;
DROP TABLE IF EXISTS provider_streams;
