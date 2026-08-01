-- +goose Up
-- Layout regions — where things are on a page (design/INGESTION_PRD.md §10 stage 1).
--
-- This is the substrate BOTH surfaces sit on (§13e): the research bench turns headings
-- into a chunk tree, and Tutor crops `figure` / `isolate_formula` / `table` regions for
-- vision transcription. One segmentation pass, two consumers — so this table stores what
-- the model saw, not either surface's interpretation of it.

CREATE TABLE public.artifact_regions (
    id          bigserial   PRIMARY KEY,
    artifact_id text        NOT NULL REFERENCES public.artifacts (id) ON DELETE CASCADE,
    page        integer     NOT NULL,          -- 1-based
    ordinal     integer     NOT NULL,          -- reading order within the page

    -- DocLayout-YOLO's DocStructBench classes. Deliberately NOT constrained by a CHECK:
    -- swapping the model is expected (§14), and a new class should land as data rather
    -- than as a failed insert.
    class       text        NOT NULL,
    confidence  real        NOT NULL DEFAULT 0,

    -- PDF POINTS, not pixels. The script converts before returning, because a box in one
    -- coordinate space and text in another is how a citation ends up pointing at the
    -- wrong place — and §13d puts zero error budget on anchors.
    x0 real NOT NULL, y0 real NOT NULL, x1 real NOT NULL, y1 real NOT NULL,

    -- The text under the box, resolved in the same pass that found the box. Redundant
    -- with artifact_pages.text by design: it is what lets the chunk tree be assembled
    -- without re-deriving every bbox lookup, and it TOASTs well.
    text        text        NOT NULL DEFAULT '',

    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX artifact_regions_reading_order_idx
    ON public.artifact_regions (artifact_id, page, ordinal);

-- Tutor asks "give me every figure in this document"; the chunk builder asks for titles.
CREATE INDEX artifact_regions_class_idx
    ON public.artifact_regions (artifact_id, class);

-- +goose Down
DROP TABLE IF EXISTS public.artifact_regions;
