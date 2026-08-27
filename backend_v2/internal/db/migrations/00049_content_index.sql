-- +goose Up
-- READABLE_WORKSPACE R1: the content index — a derived, rebuildable projection of every
-- readable artifact kind into addressable text rows. NEVER canonical: droppable and
-- rebuilt from source at any time (same philosophy as the client replica). Locators are
-- opaque strings ("page:12", "block:<id>", "shape:<id>") that clients can reopen; a hit
-- without a locator can't be cited, and citation is the point.

CREATE TABLE public.content_index (
    id          bigserial   PRIMARY KEY,
    user_id     uuid        NOT NULL,
    artifact_id text        NOT NULL REFERENCES public.artifacts (id) ON DELETE CASCADE,
    kind        text        NOT NULL,              -- artifacts.type, denormalized for filtering
    locator     text        NOT NULL DEFAULT '',   -- opaque; '' = whole-artifact row
    ordinal     integer     NOT NULL DEFAULT 0,    -- reading order within the artifact
    text        text        NOT NULL,
    -- Generated, so it can never drift from the text it indexes (00039's discipline).
    tsv         tsvector    GENERATED ALWAYS AS (to_tsvector('english', text)) STORED,
    -- Semantic layer lands at R3; NULL until an embedder fills it. No vector index yet —
    -- build it with the embedder, against real row counts.
    embedding   public.vector(1536),
    indexed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX content_index_tsv_idx      ON public.content_index USING gin (tsv);
CREATE INDEX content_index_artifact_idx ON public.content_index (artifact_id);
CREATE INDEX content_index_user_idx     ON public.content_index (user_id);

-- Per-artifact staleness bookkeeping. source_stamp is the GREATEST source clock observed
-- when the artifact was last projected — compared against artifacts / page_documents /
-- artifact_documents updated_at, because two of those are written by the sidecar with
-- direct SQL (no outbox frame), which is exactly why freshness cannot ride write-path
-- hooks alone and the sweep exists.
CREATE TABLE public.content_index_state (
    artifact_id  text        PRIMARY KEY REFERENCES public.artifacts (id) ON DELETE CASCADE,
    source_stamp timestamptz NOT NULL,
    indexed_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS public.content_index_state;
DROP TABLE IF EXISTS public.content_index;
