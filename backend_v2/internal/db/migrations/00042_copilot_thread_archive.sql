-- +goose Up
-- Thread management polish: archived threads stay durable for audit/history, but
-- disappear from the default Copilot switcher.
ALTER TABLE public.copilot_threads ADD COLUMN archived_at timestamp with time zone;
DROP INDEX IF EXISTS copilot_threads_user_idx;
CREATE INDEX copilot_threads_user_active_idx
    ON public.copilot_threads (user_id, updated_at DESC)
    WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS copilot_threads_user_active_idx;
CREATE INDEX copilot_threads_user_idx ON public.copilot_threads (user_id, updated_at DESC);
ALTER TABLE public.copilot_threads DROP COLUMN IF EXISTS archived_at;
