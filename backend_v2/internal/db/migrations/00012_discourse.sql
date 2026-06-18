-- +goose Up
-- Discourse layer (moat consolidation, ~/.claude/plans/moat-consolidation.md). buck's
-- discourse/insight engine reworked to key on the RESOLVED entity_id (the entity layer)
-- instead of the LLM-guessed canonical text it used over Neo4j. A "bridge" is a shared
-- entity mentioned by >=2 documents (derived from entity_mentions); the discourse sweep
-- analyzes the stance across those documents. Combines buck's 00008_entity_discourse +
-- 00009_discourse_insights, rekeyed on entity_id. (buck's 00006_records_vector_index and
-- 00007_artifact_enrichment are dropped/deferred.)

-- The sweep ledger: per-entity analysis state, so the sweep re-analyzes a bridge only
-- when it has GROWN (dirty by growth, not by every mention).
CREATE TABLE public.entity_discourse (
    entity_id              uuid NOT NULL,
    connected_count        integer NOT NULL DEFAULT 0,
    count_at_last_analysis integer NOT NULL DEFAULT 0,
    last_analyzed_at       timestamp with time zone,
    version                integer NOT NULL DEFAULT 0,
    updated_at             timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (entity_id),
    CONSTRAINT entity_discourse_entity_fk FOREIGN KEY (entity_id) REFERENCES public.entities(id) ON DELETE CASCADE
);

-- Discourse insights live in the existing insights table (type='discourse'), keyed on an
-- entity rather than a knowledge graph — so kg_id becomes nullable and entity_id +
-- provenance/trust/version are added. The discourse map for an entity is one row,
-- upserted on growth.
ALTER TABLE public.insights ALTER COLUMN kg_id DROP NOT NULL;
ALTER TABLE public.insights ADD COLUMN entity_id uuid;
ALTER TABLE public.insights ADD COLUMN provenance jsonb;
ALTER TABLE public.insights ADD COLUMN trust_tier text;
ALTER TABLE public.insights ADD COLUMN version integer NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX insights_discourse_entity_idx ON public.insights (entity_id) WHERE type = 'discourse';
ALTER TABLE public.insights
    ADD CONSTRAINT insights_discourse_entity_fk FOREIGN KEY (entity_id) REFERENCES public.entities(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE public.insights DROP CONSTRAINT IF EXISTS insights_discourse_entity_fk;
DROP INDEX IF EXISTS insights_discourse_entity_idx;
ALTER TABLE public.insights DROP COLUMN IF EXISTS version;
ALTER TABLE public.insights DROP COLUMN IF EXISTS trust_tier;
ALTER TABLE public.insights DROP COLUMN IF EXISTS provenance;
ALTER TABLE public.insights DROP COLUMN IF EXISTS entity_id;
ALTER TABLE public.insights ALTER COLUMN kg_id SET NOT NULL;
DROP TABLE IF EXISTS public.entity_discourse;
