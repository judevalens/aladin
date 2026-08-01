-- +goose Up
-- Ingestion, correction 1: pages become rows, not a JSON blob.
--
-- 00038 stored the whole document as `artifact_documents.pages` jsonb. Two problems
-- showed up as soon as an agent tried to use it:
--
--   1. Reading ONE page deserialized the ENTIRE document — megabytes per call for a book.
--   2. There was no way to FIND anything. The only access pattern was "give me pages
--      N..M", which forces a caller that doesn't know where to look into reading
--      sequentially — i.e. pulling a whole book through a context window.
--
-- Rows plus a full-text index fix both. Retrieval is the primitive an agent should
-- reach for; page ranges are for expanding around a hit you already have.

CREATE TABLE public.artifact_pages (
    artifact_id text    NOT NULL REFERENCES public.artifacts (id) ON DELETE CASCADE,
    page        integer NOT NULL,
    text        text    NOT NULL DEFAULT '',
    -- Generated, so it can never drift from the text it indexes.
    tsv         tsvector GENERATED ALWAYS AS (to_tsvector('english', text)) STORED,
    PRIMARY KEY (artifact_id, page)
);

CREATE INDEX artifact_pages_tsv_idx ON public.artifact_pages USING gin (tsv);

-- Carry over anything already ingested rather than forcing a re-extract.
INSERT INTO public.artifact_pages (artifact_id, page, text)
SELECT d.artifact_id,
       (elem->>'page')::int,
       COALESCE(elem->>'text', '')
  FROM public.artifact_documents d,
       LATERAL jsonb_array_elements(d.pages) AS elem
 WHERE jsonb_typeof(d.pages) = 'array'
   AND (elem->>'page') IS NOT NULL
ON CONFLICT (artifact_id, page) DO NOTHING;

ALTER TABLE public.artifact_documents DROP COLUMN pages;

-- +goose Down
ALTER TABLE public.artifact_documents ADD COLUMN pages jsonb NOT NULL DEFAULT '[]'::jsonb;

UPDATE public.artifact_documents d
   SET pages = COALESCE((
       SELECT jsonb_agg(jsonb_build_object('page', p.page, 'text', p.text) ORDER BY p.page)
         FROM public.artifact_pages p
        WHERE p.artifact_id = d.artifact_id
   ), '[]'::jsonb);

DROP TABLE IF EXISTS public.artifact_pages;
