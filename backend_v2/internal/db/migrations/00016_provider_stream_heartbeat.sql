-- +goose Up
-- Heartbeat liveness for the sync scheduler: while a worker holds a stream (queued/syncing) it
-- ticks last_heartbeat_at. ClaimBatch re-claims a stream whose heartbeat has gone stale (worker
-- crashed mid-fetch) — the missing crash-recovery backstop for streams (records already have the
-- reaper). NULL = never held.
ALTER TABLE public.provider_streams ADD COLUMN last_heartbeat_at timestamptz;

-- +goose Down
ALTER TABLE public.provider_streams DROP COLUMN last_heartbeat_at;
