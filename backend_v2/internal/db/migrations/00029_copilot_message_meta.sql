-- +goose Up
-- Per-message metadata for the assistant turn that produced it: usage/cost from
-- the SDK's result ({numTurns, inputTokens, outputTokens, costUsd}) and a compact
-- tool-activity digest ({activity:[{name, ok}]}) so replayed threads can show what
-- the turn actually did. One jsonb column instead of N scalar columns: the shape
-- is additive and only ever read whole.
ALTER TABLE public.copilot_messages ADD COLUMN meta jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE public.copilot_messages DROP COLUMN IF EXISTS meta;
