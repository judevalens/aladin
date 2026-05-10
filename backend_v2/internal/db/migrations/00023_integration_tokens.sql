-- +goose Up

CREATE TABLE IF NOT EXISTS integration_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    scopes       TEXT[] NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT 'active',
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_integration_tokens_user
    ON integration_tokens (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_integration_tokens_active_hash
    ON integration_tokens (token_hash)
    WHERE status = 'active' AND revoked_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_integration_tokens_active_hash;
DROP INDEX IF EXISTS idx_integration_tokens_user;
DROP TABLE IF EXISTS integration_tokens;
