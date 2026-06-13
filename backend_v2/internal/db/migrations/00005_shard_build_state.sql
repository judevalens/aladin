-- +goose Up
-- shard_build_state holds the latest build status per shard page + channel. It
-- powers the live build view (status chip / error overlay / auto-reload) and is
-- written transactionally with an 'app_event' outbox row, so a build-status
-- change crosses from the MCP (build) process to the API process's websocket.
-- This is ephemeral UI state, NOT part of the durable data-sync stream (the pull
-- path filters app_event rows out).
CREATE TABLE public.shard_build_state (
    page_id    text NOT NULL,
    channel    text NOT NULL,
    status     text NOT NULL,
    errors     jsonb,
    build_id   text,
    built_at   timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (page_id, channel)
);

-- +goose Down
DROP TABLE public.shard_build_state;
