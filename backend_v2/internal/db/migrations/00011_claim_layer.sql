-- +goose Up
-- The claim layer (C0 of the claims-as-first-class spec,
-- ~/.claude/plans/claims-discourse-layer.md). Claims are contestable propositions with a
-- stance, grounded in resolved entities — the opinionated unit that makes Aladin a claim
-- graph, not a note graph. C0 stands up the registry + discovered-side extraction; claim
-- resolution (C1), authored extraction (C2), and the stance/contradiction engine (C3+)
-- come later. claim_edges (the argument overlay) ships now though edges land in C3.
--
-- Mirrors the entity layer (00010): tiered scope, embedding, trust, reversible overlay.
-- pg_trgm + vector already enabled. Numbered 00011 — 00006-00009 are an uncommitted
-- branch's, already applied in the shared sandbox.

CREATE TABLE public.claims (
    id                uuid DEFAULT gen_random_uuid() NOT NULL,
    scope             text NOT NULL DEFAULT 'shared',
    owner_user_id     uuid,
    canonical_text    text NOT NULL,
    embedding         public.vector(1536),
    polarity          text NOT NULL DEFAULT 'assert',
    trust_tier        text NOT NULL DEFAULT 'believed',
    canonical_root_id uuid,
    provenance        jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamp with time zone DEFAULT now() NOT NULL,
    updated_at        timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT claims_scope_chk CHECK (scope IN ('shared', 'tenant')),
    CONSTRAINT claims_shared_no_owner_chk CHECK (scope <> 'shared' OR owner_user_id IS NULL),
    CONSTRAINT claims_polarity_chk CHECK (polarity IN ('assert', 'deny', 'neutral')),
    CONSTRAINT claims_trust_chk CHECK (trust_tier IN ('believed', 'auto', 'verified')),
    CONSTRAINT claims_root_fk FOREIGN KEY (canonical_root_id) REFERENCES public.claims(id) ON DELETE SET NULL
);
CREATE INDEX claims_scope_owner_idx ON public.claims (scope, owner_user_id);
CREATE INDEX claims_text_trgm_idx ON public.claims USING gin (canonical_text public.gin_trgm_ops);

-- What a claim is ABOUT — the grounding edges into the entity layer. Also the cheap
-- blocker for claim resolution (claims about the same entities are candidates).
CREATE TABLE public.claim_subjects (
    claim_id   uuid NOT NULL,
    entity_id  uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (claim_id, entity_id),
    CONSTRAINT claim_subjects_claim_fk FOREIGN KEY (claim_id) REFERENCES public.claims(id) ON DELETE CASCADE,
    CONSTRAINT claim_subjects_entity_fk FOREIGN KEY (entity_id) REFERENCES public.entities(id) ON DELETE CASCADE
);
CREATE INDEX claim_subjects_entity_idx ON public.claim_subjects (entity_id);

-- The evidence layer: which source asserts / denies / hedges a claim. source_id is
-- polymorphic (record | artifact) so it carries no FK; source_kind disambiguates.
CREATE TABLE public.claim_mentions (
    id          uuid DEFAULT gen_random_uuid() NOT NULL,
    claim_id    uuid NOT NULL,
    source_kind text NOT NULL,
    source_id   text NOT NULL,
    stance      text NOT NULL DEFAULT 'assert',
    confidence  double precision NOT NULL DEFAULT 1.0,
    resolver    text NOT NULL DEFAULT 'extract',
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT claim_mentions_claim_fk FOREIGN KEY (claim_id) REFERENCES public.claims(id) ON DELETE CASCADE,
    CONSTRAINT claim_mentions_stance_chk CHECK (stance IN ('assert', 'deny', 'hedge')),
    CONSTRAINT claim_mentions_unique UNIQUE (source_kind, source_id, claim_id)
);
CREATE INDEX claim_mentions_source_idx ON public.claim_mentions (source_kind, source_id);
CREATE INDEX claim_mentions_claim_idx ON public.claim_mentions (claim_id);

-- The argument overlay (supports / contradicts / qualifies between distinct claims) +
-- reversibility, exactly like entity_merges. Edges land in C3; the table ships now.
CREATE TABLE public.claim_edges (
    id            uuid DEFAULT gen_random_uuid() NOT NULL,
    from_claim_id uuid NOT NULL,
    to_claim_id   uuid NOT NULL,
    type          text NOT NULL,
    status        text NOT NULL DEFAULT 'proposed',
    confidence    double precision,
    method        text,
    evidence      jsonb NOT NULL DEFAULT '{}'::jsonb,
    decided_by    text,
    created_at    timestamp with time zone DEFAULT now() NOT NULL,
    decided_at    timestamp with time zone,
    PRIMARY KEY (id),
    CONSTRAINT claim_edges_from_fk FOREIGN KEY (from_claim_id) REFERENCES public.claims(id) ON DELETE CASCADE,
    CONSTRAINT claim_edges_to_fk FOREIGN KEY (to_claim_id) REFERENCES public.claims(id) ON DELETE CASCADE,
    CONSTRAINT claim_edges_type_chk CHECK (type IN ('supports', 'contradicts', 'qualifies')),
    CONSTRAINT claim_edges_status_chk CHECK (status IN ('proposed', 'applied', 'rejected', 'reverted')),
    CONSTRAINT claim_edges_unique UNIQUE (from_claim_id, to_claim_id, type)
);

-- +goose Down
DROP TABLE IF EXISTS public.claim_edges;
DROP TABLE IF EXISTS public.claim_mentions;
DROP TABLE IF EXISTS public.claim_subjects;
DROP TABLE IF EXISTS public.claims;
