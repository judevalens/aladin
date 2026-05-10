-- +goose Up

ALTER TABLE records
    ADD COLUMN IF NOT EXISTS provider TEXT,
    ADD COLUMN IF NOT EXISTS provider_stream_id UUID REFERENCES provider_streams(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id) ON DELETE CASCADE;

INSERT INTO records (
    id, provider, provider_stream_id, owner_user_id, external_id, source_revision,
    type, label, content, source_url, metadata, status, created_at
)
SELECT
    si.id,
    si.provider,
    si.provider_stream_id,
    si.owner_user_id,
    si.external_id,
    si.source_revision,
    si.item_type,
    si.title,
    si.content_excerpt,
    si.source_url,
    si.provider_metadata || jsonb_build_object(
        'provider', si.provider,
        'provider_stream_id', si.provider_stream_id,
        'source_revision', si.source_revision
    ),
    'captured',
    si.created_at
FROM source_items si
ON CONFLICT (id) DO UPDATE
SET provider = EXCLUDED.provider,
    provider_stream_id = EXCLUDED.provider_stream_id,
    owner_user_id = EXCLUDED.owner_user_id,
    external_id = EXCLUDED.external_id,
    source_revision = EXCLUDED.source_revision,
    type = EXCLUDED.type,
    label = EXCLUDED.label,
    content = EXCLUDED.content,
    source_url = EXCLUDED.source_url,
    metadata = EXCLUDED.metadata,
    status = CASE WHEN records.status = 'pending' THEN 'captured' ELSE records.status END
WHERE records.source_revision <= EXCLUDED.source_revision;

UPDATE records r
SET enrichment = jsonb_build_object(
        'summary', e.summary,
        'entities', e.entities,
        'topics', e.topics,
        'key_claims', e.key_claims,
        'low_confidence_entities', e.low_confidence_entities,
        'status', e.status
    ),
    embedding = COALESCE(e.embedding, r.embedding),
    status = CASE WHEN r.status IN ('pending', 'captured') THEN 'enriched' ELSE r.status END
FROM source_item_enrichments e
WHERE r.id = e.source_item_id
  AND r.source_revision = e.source_revision;

ALTER TABLE tenant_item_matches
    DROP CONSTRAINT IF EXISTS tenant_item_matches_record_id_fkey;

UPDATE tenant_item_matches
SET record_id = source_item_id
WHERE source_item_id IS NOT NULL;

ALTER TABLE tenant_item_matches
    DROP CONSTRAINT IF EXISTS tenant_item_matches_subscription_id_source_item_id_source_revision_key;

ALTER TABLE tenant_item_matches
    DROP CONSTRAINT IF EXISTS tenant_item_matches_subscription_id_source_item_id_source_revisi_key;

ALTER TABLE tenant_item_matches
    ALTER COLUMN record_id SET NOT NULL;

ALTER TABLE tenant_item_matches
    ADD CONSTRAINT tenant_item_matches_record_id_fkey
    FOREIGN KEY (record_id) REFERENCES records(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_item_matches_subscription_record_revision
    ON tenant_item_matches (subscription_id, record_id, source_revision);

ALTER TABLE tenant_item_matches
    DROP COLUMN IF EXISTS source_item_id;

DROP TABLE IF EXISTS source_item_enrichments;
DROP TABLE IF EXISTS source_items;

-- +goose Down

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

ALTER TABLE tenant_item_matches
    ADD COLUMN IF NOT EXISTS source_item_id TEXT REFERENCES source_items(id) ON DELETE CASCADE;

UPDATE tenant_item_matches
SET source_item_id = record_id
WHERE source_item_id IS NULL;

DROP INDEX IF EXISTS uq_tenant_item_matches_subscription_record_revision;

ALTER TABLE tenant_item_matches
    ALTER COLUMN source_item_id SET NOT NULL;

ALTER TABLE tenant_item_matches
    ALTER COLUMN record_id DROP NOT NULL;

ALTER TABLE tenant_item_matches
    ADD CONSTRAINT tenant_item_matches_subscription_id_source_item_revision_key
    UNIQUE (subscription_id, source_item_id, source_revision);

ALTER TABLE records
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS provider_stream_id,
    DROP COLUMN IF EXISTS provider;
