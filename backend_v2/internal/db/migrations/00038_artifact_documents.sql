-- +goose Up
-- Ingestion v1 (design/INGESTION_PRD.md): make a dropped document readable.
--
-- Uploading a PDF stored bytes on disk and nothing else — the file was unreadable,
-- unsearchable, and invisible to agents. These two tables hold what extraction
-- recovers: the text, and the structure.
--
-- Deliberately NOT here: entities, topics, claims, summaries. §2 of the PRD — ingestion
-- extracts, it does not interpret. Text and structure are facts about the file;
-- everything else is an opinion about its meaning and belongs to a later, separate step
-- that reads these rows.

CREATE TABLE public.artifact_documents (
    artifact_id text        PRIMARY KEY REFERENCES public.artifacts (id) ON DELETE CASCADE,
    user_id     uuid        NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,

    -- §4: status is first-class, because extraction fails in ways worth telling apart.
    -- 'unsupported' is the load-bearing one: a scanned PDF has no text layer, which is
    -- not a crash and not a success — it's a document that needs OCR first.
    status      text        NOT NULL DEFAULT 'pending',
    error       text,

    page_count  integer     NOT NULL DEFAULT 0,

    -- Per-page text, so a section (which knows only its page) can resolve to its words.
    -- Lives in Postgres and NOT on the sync spine: a book is megabytes and the tree
    -- frame carries light fields only. The node frame carries status; this is fetched
    -- when the document is opened.
    pages       jsonb       NOT NULL DEFAULT '[]'::jsonb,

    -- Which extractor produced this, so a better one later can be detected and re-run.
    extractor   text        NOT NULL DEFAULT '',

    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT artifact_documents_status_chk
        CHECK (status IN ('pending', 'ingesting', 'ready', 'unsupported', 'failed'))
);

CREATE INDEX artifact_documents_user_status_idx
    ON public.artifact_documents (user_id, status);

-- The outline. Populated from the PDF's OWN bookmarks — §5: if a document doesn't carry
-- a structure we leave this empty rather than inventing one and presenting it as the
-- document's. Generating an outline (tools/pdftoc) is v2, and keeps its editable step.
CREATE TABLE public.artifact_sections (
    id          bigserial   PRIMARY KEY,
    artifact_id text        NOT NULL REFERENCES public.artifacts (id) ON DELETE CASCADE,
    title       text        NOT NULL,
    level       integer     NOT NULL DEFAULT 0,
    page        integer     NOT NULL,          -- 1-based PDF page
    position    integer     NOT NULL,          -- document order; the outline is a sequence
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX artifact_sections_artifact_idx
    ON public.artifact_sections (artifact_id, position);

-- +goose Down
DROP TABLE IF EXISTS public.artifact_sections;
DROP TABLE IF EXISTS public.artifact_documents;
