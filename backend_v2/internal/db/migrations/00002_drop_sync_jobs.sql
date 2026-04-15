-- +goose Up
DROP TABLE IF EXISTS sync_jobs;

-- +goose Down
CREATE TABLE IF NOT EXISTS sync_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id    UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    snapshot_id  UUID REFERENCES snapshots(id) ON DELETE CASCADE,
    source_type  TEXT NOT NULL,
    job_type     TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    priority     INTEGER NOT NULL DEFAULT 1,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error   TEXT,
    status       TEXT NOT NULL DEFAULT 'pending'
);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_dequeue ON sync_jobs (priority, scheduled_at, created_at) WHERE status = 'pending';
