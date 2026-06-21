-- +goose Up
-- Pipeline observability (M9 I4): records need a stage-change timestamp so the stats
-- surface can compute in-flight age, stuck records, completion throughput, and end-to-end
-- latency. created_at is ingestion time; updated_at advances on every status transition
-- (captured → enriched → complete | failed).
ALTER TABLE public.records ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS records_status_updated_idx ON public.records (status, updated_at);

-- +goose Down
DROP INDEX IF EXISTS records_status_updated_idx;
ALTER TABLE public.records DROP COLUMN IF EXISTS updated_at;
