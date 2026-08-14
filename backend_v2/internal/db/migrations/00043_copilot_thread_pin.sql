-- +goose Up
-- Pinned Copilot threads stay at the top of the switcher while still sorting by
-- recent activity within the pinned/unpinned groups.
ALTER TABLE public.copilot_threads ADD COLUMN pinned_at timestamp with time zone;
DROP INDEX IF EXISTS copilot_threads_user_active_idx;
CREATE INDEX copilot_threads_user_active_idx
    ON public.copilot_threads (user_id, pinned_at DESC, updated_at DESC)
    WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS copilot_threads_user_active_idx;
CREATE INDEX copilot_threads_user_active_idx
    ON public.copilot_threads (user_id, updated_at DESC)
    WHERE archived_at IS NULL;
ALTER TABLE public.copilot_threads DROP COLUMN IF EXISTS pinned_at;
