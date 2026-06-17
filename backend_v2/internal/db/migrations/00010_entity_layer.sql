-- +goose Up
-- The entity layer (R0 of the entity-resolution spec, ~/.claude/plans/entity-resolution-dedup.md).
-- Turns the free-text entity strings on records.enrichment into a real registry with
-- stable ids, so "OpenAI", "OpenAI Inc", "OpenAI, Inc." resolve to ONE entity instead
-- of three. R0 populates the shared (Tier 0) scope from public ingestion; the
-- scope / owner_user_id / shared_entity_id columns ship now so the per-tenant tier (R3)
-- is not an expensive retrofit. entity_merges is created now (the reversible-merge
-- overlay) even though merges land in R1.
--
-- Numbered 00010 deliberately: 00006-00009 are claimed by an uncommitted branch and are
-- already applied in the shared sandbox DB — reusing them would be silently skipped.

CREATE TABLE public.entities (
    id                uuid DEFAULT gen_random_uuid() NOT NULL,
    scope             text NOT NULL DEFAULT 'shared',
    owner_user_id     uuid,
    shared_entity_id  uuid,
    kind              text NOT NULL DEFAULT 'unknown',
    canonical_name    text NOT NULL,
    normalized_key    text NOT NULL,
    canonical_root_id uuid,
    trust_tier        text NOT NULL DEFAULT 'believed',
    embedding         public.vector(1536),
    disambiguators    jsonb NOT NULL DEFAULT '{}'::jsonb,
    provenance        jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamp with time zone DEFAULT now() NOT NULL,
    updated_at        timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT entities_scope_chk CHECK (scope IN ('shared', 'tenant')),
    -- invariant: a shared (Tier 0) entity is system-owned and never points up at a tenant.
    CONSTRAINT entities_shared_no_owner_chk CHECK (scope <> 'shared' OR (owner_user_id IS NULL AND shared_entity_id IS NULL)),
    CONSTRAINT entities_shared_fk FOREIGN KEY (shared_entity_id) REFERENCES public.entities(id) ON DELETE SET NULL,
    CONSTRAINT entities_root_fk FOREIGN KEY (canonical_root_id) REFERENCES public.entities(id) ON DELETE SET NULL
);
-- Blocking is always within a scope (a tenant's overlay or the shared tier), never across owners.
CREATE INDEX entities_scope_owner_kind_key_idx ON public.entities (scope, owner_user_id, kind, normalized_key);
CREATE INDEX entities_name_trgm_idx ON public.entities USING gin (canonical_name public.gin_trgm_ops);
CREATE INDEX entities_shared_entity_idx ON public.entities (shared_entity_id);

CREATE TABLE public.entity_aliases (
    id         uuid DEFAULT gen_random_uuid() NOT NULL,
    entity_id  uuid NOT NULL,
    surface    text NOT NULL,
    normalized text NOT NULL,
    kind       text NOT NULL DEFAULT 'unknown',
    source     text NOT NULL DEFAULT 'enrichment',
    confidence double precision NOT NULL DEFAULT 1.0,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT entity_aliases_entity_fk FOREIGN KEY (entity_id) REFERENCES public.entities(id) ON DELETE CASCADE,
    CONSTRAINT entity_aliases_unique UNIQUE (entity_id, normalized)
);
CREATE INDEX entity_aliases_kind_norm_idx ON public.entity_aliases (kind, normalized);

CREATE TABLE public.entity_mentions (
    id              uuid DEFAULT gen_random_uuid() NOT NULL,
    record_id       text NOT NULL,
    entity_id       uuid NOT NULL,
    surface         text NOT NULL,
    kind            text NOT NULL DEFAULT 'unknown',
    confidence      double precision NOT NULL DEFAULT 1.0,
    resolver        text NOT NULL,
    source_revision bigint NOT NULL DEFAULT 0,
    created_at      timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT entity_mentions_record_fk FOREIGN KEY (record_id) REFERENCES public.records(id) ON DELETE CASCADE,
    CONSTRAINT entity_mentions_entity_fk FOREIGN KEY (entity_id) REFERENCES public.entities(id) ON DELETE CASCADE,
    CONSTRAINT entity_mentions_unique UNIQUE (record_id, entity_id, surface)
);
CREATE INDEX entity_mentions_entity_idx ON public.entity_mentions (entity_id);
CREATE INDEX entity_mentions_record_idx ON public.entity_mentions (record_id);

-- The reversible-merge overlay. Mentions bind to the entity they resolved to and are
-- never rewritten; merges are a mutable layer on top, so a wrong merge is undoable.
CREATE TABLE public.entity_merges (
    id             uuid DEFAULT gen_random_uuid() NOT NULL,
    from_entity_id uuid NOT NULL,
    into_entity_id uuid NOT NULL,
    status         text NOT NULL DEFAULT 'proposed',
    confidence     double precision,
    method         text,
    evidence       jsonb NOT NULL DEFAULT '{}'::jsonb,
    decided_by     text,
    created_at     timestamp with time zone DEFAULT now() NOT NULL,
    decided_at     timestamp with time zone,
    PRIMARY KEY (id),
    CONSTRAINT entity_merges_from_fk FOREIGN KEY (from_entity_id) REFERENCES public.entities(id) ON DELETE CASCADE,
    CONSTRAINT entity_merges_into_fk FOREIGN KEY (into_entity_id) REFERENCES public.entities(id) ON DELETE CASCADE,
    CONSTRAINT entity_merges_status_chk CHECK (status IN ('proposed', 'applied', 'rejected', 'reverted')),
    CONSTRAINT entity_merges_unique UNIQUE (from_entity_id, into_entity_id)
);

-- +goose Down
DROP TABLE IF EXISTS public.entity_merges;
DROP TABLE IF EXISTS public.entity_mentions;
DROP TABLE IF EXISTS public.entity_aliases;
DROP TABLE IF EXISTS public.entities;
