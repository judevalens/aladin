-- +goose Up
-- Entity Context surface, Phase B (plan: ~/.claude/plans/entity-context-surface.md;
-- product intent: design/ENTITY_CONTEXT_PRD.md).
--
-- 1) entities.gist — the one-sentence definition the surface shows under the title. Per
--    the PRD this is "the only prose the page authors"; everything else on the page is
--    verbatim user/source material. Nullable: most entities never get one.
--
-- 2) relationships becomes the entity edge layer. The table (00006) was built as an
--    ADDITIVE polymorphic edge layer over artifact/record/insight, deliberately without
--    FKs on its endpoints. Entities are simply a new endpoint kind — but the CHECK
--    constraints excluded them, so they are widened here rather than adding a parallel
--    edge table.
--
--    The 7 typed relations from the PRD join the 5 existing ones. `supports` and
--    `contradicts` already existed and are REUSED (same meaning, new endpoint kind).
--    Values are snake_case, consistent with the existing `derived_from`.
--
--    Directional and typed: (src)-[rel]->(dst). The `why` (the user's reasoning — the
--    substance of the edge) and `origin` ('you' | 'watcher' → the YOURS/FOUND tag) live
--    in the existing `metadata` jsonb, keeping this migration additive.
--
--    Endpoint integrity stays at the service layer (00006's design: entity ids are uuids,
--    artifact ids are text, so no single FK spans them).
--
-- Existing indexes already serve entity edges in both directions:
-- idx_relationships_src/(user_id, src_kind, src_id) and the dst twin; uq_relationships_edge
-- keeps (user, src, dst, rel_type) unique, so drawing the same edge twice is a no-op.

-- 3) artifact_entities.snippet — the plain text of the block an @mention sits in,
--    captured when the editor projects its mentions. Without it we know THAT a page
--    mentions an entity but not what it SAID, so the Context section could never render
--    "your note" (the PRD's brightest, most important context type — the user's own
--    thinking). Verbatim by construction: it is the block's text, never rewritten.
--    Nullable — tag-origin rows and pre-existing mention rows have none.

ALTER TABLE public.entities ADD COLUMN IF NOT EXISTS gist text;
ALTER TABLE public.artifact_entities ADD COLUMN IF NOT EXISTS snippet text;

ALTER TABLE public.relationships DROP CONSTRAINT IF EXISTS relationships_src_kind_chk;
ALTER TABLE public.relationships ADD CONSTRAINT relationships_src_kind_chk
    CHECK (src_kind IN ('artifact', 'record', 'insight', 'entity'));

ALTER TABLE public.relationships DROP CONSTRAINT IF EXISTS relationships_dst_kind_chk;
ALTER TABLE public.relationships ADD CONSTRAINT relationships_dst_kind_chk
    CHECK (dst_kind IN ('artifact', 'record', 'insight', 'entity'));

ALTER TABLE public.relationships DROP CONSTRAINT IF EXISTS relationships_type_chk;
ALTER TABLE public.relationships ADD CONSTRAINT relationships_type_chk
    CHECK (rel_type IN (
        -- 00006 (artifact/record/insight bridge)
        'cites', 'about', 'derived_from',
        -- shared by both layers
        'supports', 'contradicts',
        -- entity edges (Entity Context PRD §3)
        'enables', 'enabled_by', 'part_of', 'instance', 'competes'
    ));

-- +goose Down
-- NOTE: narrowing the CHECKs back will fail if any entity edges exist (by design —
-- the constraint is doing its job). Delete src_kind/dst_kind='entity' rows first.
ALTER TABLE public.relationships DROP CONSTRAINT IF EXISTS relationships_type_chk;
ALTER TABLE public.relationships ADD CONSTRAINT relationships_type_chk
    CHECK (rel_type IN ('cites', 'supports', 'contradicts', 'about', 'derived_from'));

ALTER TABLE public.relationships DROP CONSTRAINT IF EXISTS relationships_dst_kind_chk;
ALTER TABLE public.relationships ADD CONSTRAINT relationships_dst_kind_chk
    CHECK (dst_kind IN ('artifact', 'record', 'insight'));

ALTER TABLE public.relationships DROP CONSTRAINT IF EXISTS relationships_src_kind_chk;
ALTER TABLE public.relationships ADD CONSTRAINT relationships_src_kind_chk
    CHECK (src_kind IN ('artifact', 'record', 'insight'));

ALTER TABLE public.artifact_entities DROP COLUMN IF EXISTS snippet;
ALTER TABLE public.entities DROP COLUMN IF EXISTS gist;
