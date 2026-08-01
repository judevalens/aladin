-- +goose Up
-- The chunk tree (design/INGESTION_PRD.md §11).
--
-- §11 is explicit that this is a TREE and not a partition: a chapter is a chunk AND
-- contains chunks. Flattening to leaves would throw away the structure segmentation was
-- run to recover, and it would cost multi-resolution retrieval — matching coarse for
-- "what is this about" and fine for "what exactly does it claim".

CREATE TABLE public.artifact_chunks (
    id          bigserial   PRIMARY KEY,
    artifact_id text        NOT NULL REFERENCES public.artifacts (id) ON DELETE CASCADE,

    -- Self-referencing: NULL = a top-level node. Parents are always written before their
    -- children (ingestion.FlattenChunks walks depth-first) so this resolves on insert.
    parent_id   bigint      REFERENCES public.artifact_chunks (id) ON DELETE CASCADE,

    ordinal     integer     NOT NULL,          -- order among siblings
    depth       integer     NOT NULL DEFAULT 0,
    kind        text        NOT NULL,          -- 'section' (internal) | 'block' (leaf)
    title       text        NOT NULL DEFAULT '', -- sections only

    -- Page spans are the UNION of the regions that formed the chunk — observed, never
    -- computed. §13d puts the entire error budget on boundaries and none on anchors: a
    -- section claiming the wrong pages is a confident false citation.
    page_from   integer     NOT NULL,
    page_to     integer     NOT NULL,

    text        text        NOT NULL DEFAULT '',
    -- Chunk-level search, so a hit returns a passage rather than a whole page.
    tsv         tsvector    GENERATED ALWAYS AS (to_tsvector('english', text)) STORED,

    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT artifact_chunks_kind_chk CHECK (kind IN ('section', 'block')),
    CONSTRAINT artifact_chunks_span_chk CHECK (page_to >= page_from)
);

CREATE INDEX artifact_chunks_tree_idx  ON public.artifact_chunks (artifact_id, parent_id, ordinal);
CREATE INDEX artifact_chunks_tsv_idx   ON public.artifact_chunks USING gin (tsv);
-- "what covers page 47" — the lookup that turns a search hit into its surrounding section.
CREATE INDEX artifact_chunks_pages_idx ON public.artifact_chunks (artifact_id, page_from, page_to);

-- +goose Down
DROP TABLE IF EXISTS public.artifact_chunks;
