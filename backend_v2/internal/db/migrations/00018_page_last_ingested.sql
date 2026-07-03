-- +goose Up
-- You-stream ingestion (Y1): track the page_documents revision last folded into the
-- knowledge engine. The idle-sweep only re-ingests a page once it has edits past this
-- mark, and the worker gates out stale/duplicate runs against it. 0 = never ingested.
ALTER TABLE public.page_documents ADD COLUMN IF NOT EXISTS last_ingested_revision bigint NOT NULL DEFAULT 0;

-- Sweep predicate: pages whose content moved past the last ingest, ordered by edit time.
-- Partial index keeps the idle-sweep cheap (only un-ingested pages are indexed).
CREATE INDEX IF NOT EXISTS page_documents_pending_ingest_idx
    ON public.page_documents (updated_at)
    WHERE revision > last_ingested_revision;

-- +goose Down
DROP INDEX IF EXISTS page_documents_pending_ingest_idx;
ALTER TABLE public.page_documents DROP COLUMN IF EXISTS last_ingested_revision;
