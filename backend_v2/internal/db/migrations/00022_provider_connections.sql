-- +goose Up

CREATE TABLE IF NOT EXISTS provider_connections (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider               TEXT NOT NULL,
    backend                TEXT NOT NULL DEFAULT 'nango',
    provider_config_key    TEXT NOT NULL,
    external_connection_id TEXT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'active',
    granted_scopes         TEXT[] NOT NULL DEFAULT '{}',
    metadata               JSONB NOT NULL DEFAULT '{}',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_connections_active
    ON provider_connections (user_id, backend, provider_config_key, external_connection_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_provider_connections_user_active
    ON provider_connections (user_id, updated_at DESC)
    WHERE status = 'active';

ALTER TABLE provider_streams
    ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS provider_connection_id UUID REFERENCES provider_connections(id) ON DELETE SET NULL;

ALTER TABLE provider_streams
    DROP CONSTRAINT IF EXISTS provider_streams_provider_stream_kind_stream_key_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_streams_public
    ON provider_streams (provider, stream_kind, stream_key)
    WHERE owner_user_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_streams_private_owner
    ON provider_streams (provider, stream_kind, stream_key, owner_user_id)
    WHERE owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_provider_streams_owner_connection
    ON provider_streams (owner_user_id, provider_connection_id)
    WHERE owner_user_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_provider_streams_owner_connection;
DROP INDEX IF EXISTS uq_provider_streams_private_owner;
DROP INDEX IF EXISTS uq_provider_streams_public;

ALTER TABLE provider_streams
    DROP COLUMN IF EXISTS provider_connection_id,
    DROP COLUMN IF EXISTS owner_user_id;

ALTER TABLE provider_streams
    ADD CONSTRAINT provider_streams_provider_stream_kind_stream_key_key
    UNIQUE (provider, stream_kind, stream_key);

DROP INDEX IF EXISTS idx_provider_connections_user_active;
DROP INDEX IF EXISTS uq_provider_connections_active;
DROP TABLE IF EXISTS provider_connections;
