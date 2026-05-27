-- +goose Up
-- +goose StatementBegin

-- M5: Cut page storage over from markdown to BlockNote JSON.
--
-- Test workspace; existing page data is wiped rather than backfilled.
-- The TRUNCATE on artifacts cascades to page_documents and tree_nodes
-- via their ON DELETE CASCADE references.

TRUNCATE TABLE artifacts RESTART IDENTITY CASCADE;

ALTER TABLE page_documents DROP COLUMN IF EXISTS markdown;

ALTER TABLE page_documents
    ADD COLUMN blocks JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE page_documents
    ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

-- Pure-substring search via ILIKE for v1; ranking/FTS is a follow-up.
-- A trigram index keeps ILIKE fast as content grows.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_page_documents_search_text_trgm
    ON page_documents USING gin (search_text gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_page_documents_search_text_trgm;

ALTER TABLE page_documents DROP COLUMN IF EXISTS search_text;
ALTER TABLE page_documents DROP COLUMN IF EXISTS blocks;

ALTER TABLE page_documents
    ADD COLUMN markdown TEXT NOT NULL DEFAULT '';

-- +goose StatementEnd
