-- +goose Up
-- The copilot moved from a hand-rolled in-process loop to the Claude Agent SDK
-- (Node sidecar). The SDK keeps its own session transcript (tool calls included)
-- keyed by a session id; storing it on the thread lets each new turn resume the
-- prior session instead of replaying text history. Nullable: legacy threads and
-- resume-failure fallbacks simply start a fresh session (the id is re-stamped
-- after every turn — resume forks a new session id each time).
ALTER TABLE public.copilot_threads ADD COLUMN sdk_session_id text;

-- +goose Down
ALTER TABLE public.copilot_threads DROP COLUMN IF EXISTS sdk_session_id;
