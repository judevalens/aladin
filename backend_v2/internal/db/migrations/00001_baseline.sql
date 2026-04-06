-- +goose Up

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
    id          TEXT PRIMARY KEY,
    filename    TEXT NOT NULL,
    title       TEXT,
    size        INTEGER,
    page_count  INTEGER,
    uploaded_at TIMESTAMPTZ NOT NULL,
    ingested    BOOLEAN NOT NULL,
    uploaded_by UUID REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS chunks (
    id          UUID PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id),
    page        INTEGER,
    text        TEXT NOT NULL,
    embedding   vector(1536)
);

CREATE TABLE IF NOT EXISTS knowledge_graphs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id),
    name                TEXT NOT NULL,
    description         TEXT,
    pipeline_autonomy   TEXT NOT NULL DEFAULT 'suggest',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

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
    artifact_count  INTEGER NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_id, version)
);

CREATE TABLE IF NOT EXISTS sync_cycles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id     UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active',
    cursor        JSONB NOT NULL DEFAULT '{}',
    covered_until JSONB NOT NULL DEFAULT '{}',
    last_picked_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS artifacts (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL,
    label           TEXT NOT NULL,
    content         TEXT NOT NULL,
    source_url      TEXT,
    enrichment      JSONB,
    source_id       UUID REFERENCES sources(id) ON DELETE SET NULL,
    snapshot_id     UUID REFERENCES snapshots(id) ON DELETE SET NULL,
    external_id     TEXT,
    version         INTEGER NOT NULL DEFAULT 1,
    superseded_by   TEXT REFERENCES artifacts(id) ON DELETE SET NULL,
    embedding       vector(1536),
    metadata        JSONB NOT NULL DEFAULT '{}',
    relevance_score FLOAT,
    status          TEXT NOT NULL DEFAULT 'pending',
    parent_id       TEXT REFERENCES artifacts(id) ON DELETE CASCADE,
    user_status     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS insights (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kg_id       UUID NOT NULL REFERENCES knowledge_graphs(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    artifact_ids JSONB NOT NULL DEFAULT '[]',
    entity      TEXT,
    topic       TEXT,
    confidence  FLOAT NOT NULL DEFAULT 0.8,
    user_status TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ DEFAULT now()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_sources_due         ON sources (next_sync_at) WHERE sync_mode = 'poll' AND sync_state = 'active';
CREATE INDEX IF NOT EXISTS idx_sources_claimable   ON sources (last_synced_at) WHERE sync_mode = 'poll' AND sync_state = 'active' AND sync_status = 'idle';
CREATE INDEX IF NOT EXISTS idx_sources_scheduler   ON sources (last_picked_at, last_synced_at, created_at) WHERE sync_mode = 'poll' AND sync_state = 'active' AND sync_status = 'idle';
CREATE INDEX IF NOT EXISTS idx_sync_cycles_source_active ON sync_cycles (source_id, created_at) WHERE status IN ('active', 'running');
CREATE INDEX IF NOT EXISTS idx_snapshots_source    ON snapshots (source_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_source    ON artifacts (source_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_snapshot  ON artifacts (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_external  ON artifacts (source_id, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_artifacts_pending   ON artifacts (created_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_artifacts_parent    ON artifacts (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_artifacts_top_level ON artifacts (source_id, created_at) WHERE parent_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_insights_kg          ON insights (kg_id);
CREATE INDEX IF NOT EXISTS idx_insights_user_status ON insights (user_status);
CREATE INDEX IF NOT EXISTS idx_artifacts_user_status ON artifacts (user_status) WHERE user_status IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_artifacts_pipeline_status ON artifacts (status, created_at) WHERE status IN ('pending', 'enriched', 'embedded');

CREATE UNIQUE INDEX IF NOT EXISTS uq_artifacts_source_external ON artifacts (source_id, external_id) WHERE external_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_artifacts_source_external;
DROP INDEX IF EXISTS idx_artifacts_pipeline_status;
DROP INDEX IF EXISTS idx_artifacts_user_status;
DROP INDEX IF EXISTS idx_insights_user_status;
DROP INDEX IF EXISTS idx_insights_kg;
DROP INDEX IF EXISTS idx_artifacts_top_level;
DROP INDEX IF EXISTS idx_artifacts_parent;
DROP INDEX IF EXISTS idx_artifacts_pending;
DROP INDEX IF EXISTS idx_artifacts_external;
DROP INDEX IF EXISTS idx_artifacts_snapshot;
DROP INDEX IF EXISTS idx_artifacts_source;
DROP INDEX IF EXISTS idx_snapshots_source;
DROP INDEX IF EXISTS idx_sources_claimable;
DROP INDEX IF EXISTS idx_sources_scheduler;
DROP INDEX IF EXISTS idx_sources_due;
DROP INDEX IF EXISTS idx_sync_cycles_source_active;
DROP TABLE IF EXISTS insights;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS sync_cycles;
DROP TABLE IF EXISTS snapshots;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS knowledge_graphs;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS users;
