-- +goose Up
-- Entity tags + @entity mentions (~/.claude/plans/entity-tags-mentions.md). A direct,
-- human-curated artifact↔entity link — the deterministic counterpart to the (unwired)
-- LLM authored-extraction path. `entity_mentions` is hard-FK'd to records, so pages
-- need their own link. One table, two entry points:
--   tag     → page-level metadata        (origin='tag',     block_id NULL)
--   mention → inline @entity, located     (origin='mention', block_id set, projected
--             out of the BlockNote ydoc on save)
-- Separate from claim_mentions: a tag/mention is "this page is about X", not a claim
-- with a stance.
CREATE TABLE public.artifact_entities (
    id          uuid        NOT NULL DEFAULT gen_random_uuid(),
    artifact_id text        NOT NULL,
    entity_id   uuid        NOT NULL,
    origin      text        NOT NULL DEFAULT 'tag',
    block_id    text,
    surface     text,
    added_by    uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT artifact_entities_origin_check CHECK (origin IN ('tag', 'mention')),
    CONSTRAINT artifact_entities_artifact_fk FOREIGN KEY (artifact_id) REFERENCES public.artifacts(id) ON DELETE CASCADE,
    CONSTRAINT artifact_entities_entity_fk FOREIGN KEY (entity_id) REFERENCES public.entities(id) ON DELETE CASCADE
);
-- one tag per (page, entity); mentions dedupe by (page, entity, block, surface). COALESCE
-- so a tag's NULL block_id/surface still collide on a single row (NULLs are otherwise
-- distinct in a unique key).
CREATE UNIQUE INDEX artifact_entities_unique ON public.artifact_entities
    (artifact_id, entity_id, origin, COALESCE(block_id, ''), COALESCE(surface, ''));
CREATE INDEX artifact_entities_artifact_idx ON public.artifact_entities (artifact_id);
CREATE INDEX artifact_entities_entity_idx ON public.artifact_entities (entity_id);

-- +goose Down
DROP TABLE IF EXISTS public.artifact_entities;
