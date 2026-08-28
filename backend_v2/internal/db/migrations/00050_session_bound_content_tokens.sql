-- Shard credentials share the lifetime of the login session that minted them.
-- Legacy credentials record only a user, so their issuing session cannot be
-- recovered safely. Retire those credentials; keep users, sessions and shards.
-- Relaunch the updated client after deployment to clear cached legacy URLs.

-- +goose Up
DELETE FROM content_tokens;

ALTER TABLE content_tokens
    ADD COLUMN session_token_hash text NOT NULL
    REFERENCES user_sessions(token_hash) ON DELETE CASCADE;

CREATE INDEX content_tokens_session_idx ON content_tokens (session_token_hash);

-- +goose Down
-- Do not leave session-lifetime tokens usable by old, session-unaware code.
DELETE FROM content_tokens;
ALTER TABLE content_tokens DROP COLUMN session_token_hash;
