-- Content tokens — the scoped credential the shard iframe carries.
--
-- A shard is served at /content/<id>/?access_token=…; that token is READABLE BY
-- SHARD JS (it is in the frame's own URL) and the shard CSP allows outbound
-- connections. Until now that token was the viewer's FULL session bearer, so a
-- shard could call /api as the user. A content token authenticates the same user
-- but carries only the content:read scope, and the auth middleware rejects it on
-- every non-/content route — so the worst a shard can do with its own URL is
-- re-fetch itself. Short-lived; minted on demand by the app host.

-- +goose Up
CREATE TABLE content_tokens (
    token_hash text PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX content_tokens_user_idx ON content_tokens (user_id, expires_at DESC);

-- +goose Down
DROP TABLE content_tokens;
