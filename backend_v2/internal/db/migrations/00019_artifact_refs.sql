-- +goose Up
-- Y2 — cross-reference layer. A `#` reference in a page hand-links a block to an existing
-- claim OR another artifact (a page or a shard). The twin of artifact_entities (@entity):
-- entity mentions say "this page is ABOUT X"; refs say "this block POINTS AT Y". Polymorphic
-- target: target_kind picks the table, target_id is the claim uuid (as text) or the
-- artifact id. No FK — the target lives in one of two tables (same shape as claim_mentions).
-- Only target_kind='claim' rows feed the discourse engine; page/shard rows are navigation
-- + graph backlinks.
CREATE TABLE IF NOT EXISTS public.artifact_refs (
    id          uuid        NOT NULL DEFAULT gen_random_uuid(),
    artifact_id text        NOT NULL,                       -- the page CONTAINING the ref
    target_kind text        NOT NULL,                       -- 'claim' | 'page' | 'shard'
    target_id   text        NOT NULL,                       -- claim uuid (text) or artifact id
    block_id    text,                                       -- BlockNote block the ref sits in
    surface     text,                                       -- literal label shown in the chip
    origin      text        NOT NULL DEFAULT 'reference',   -- room for future 'auto' refs
    added_by    uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT artifact_refs_kind_check   CHECK (target_kind IN ('claim', 'page', 'shard')),
    CONSTRAINT artifact_refs_origin_check CHECK (origin IN ('reference'))
);

-- One row per (page, target, block, surface) — makes the reconcile insert idempotent.
CREATE UNIQUE INDEX IF NOT EXISTS artifact_refs_unique ON public.artifact_refs
    (artifact_id, target_kind, target_id, COALESCE(block_id, ''), COALESCE(surface, ''));

-- Outgoing refs of a page (reconcile + hydrate the editor).
CREATE INDEX IF NOT EXISTS artifact_refs_artifact_idx ON public.artifact_refs (artifact_id);

-- Incoming refs to a target (backlinks: "what points at this claim/page/shard").
CREATE INDEX IF NOT EXISTS artifact_refs_target_idx ON public.artifact_refs (target_kind, target_id);

-- +goose Down
DROP TABLE IF EXISTS public.artifact_refs;
