-- +goose Up
-- Person + Company entity model (project_trading_entity_data_model). Three additions,
-- all on the existing thin `entities` table — `kind` stays a soft convention, no per-type
-- schema:
--
--   1) entities.data_points — the {name,type,value|id} typed data-point map (mirrors the
--      shipped ArtifactProperty shape). Sparse/subjective attributes live here; `reference`
--      values (id → row) are a fast-follow. Nullable-by-default '[]'.
--   2) entity_external_ids — HARD, exact-match cross-system keys (CIK/LEI/CUSIP/…). The
--      deterministic tier of the resolver; UNIQUE(system,value) prevents company twins.
--      Kept SEPARATE from entity_aliases (fuzzy names) — opposite matching semantics.
--   3) companies — the 1:1 HARD extension table for kind='company': the objective facts
--      you rank across the universe (sector/industry/employees/…) as real columns, NOT
--      json. Person gets NO extension table (sparse → data points), per the design.
--
-- 'company' is a distinct kind (specialization of 'org'): it carries a CIK, links to its
-- securities via issued_by (fast-follow), and owns the companies row. `entities.kind` has
-- no CHECK (free text since 00020), so no constraint change — we just start using it.

ALTER TABLE public.entities ADD COLUMN IF NOT EXISTS data_points jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE public.entity_external_ids (
    id         uuid DEFAULT gen_random_uuid() NOT NULL,
    entity_id  uuid NOT NULL,
    system     text NOT NULL,           -- 'cik' | 'lei' | 'cusip' | 'isin' | 'figi' | ...
    value      text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT entity_external_ids_entity_fk FOREIGN KEY (entity_id)
        REFERENCES public.entities(id) ON DELETE CASCADE,
    -- The deterministic-resolution invariant: one (system,value) identity → one entity.
    CONSTRAINT entity_external_ids_uq UNIQUE (system, value)
);
CREATE INDEX entity_external_ids_entity_idx ON public.entity_external_ids (entity_id);
CREATE INDEX entity_external_ids_lookup_idx ON public.entity_external_ids (system, value);

CREATE TABLE public.companies (
    entity_id    uuid NOT NULL,          -- 1:1 with a kind='company' entity (surrogate join, NOT the CIK)
    sector       text NOT NULL DEFAULT '',
    industry     text NOT NULL DEFAULT '',
    description  text NOT NULL DEFAULT '',
    website      text NOT NULL DEFAULT '',
    country      text NOT NULL DEFAULT '',
    employees    integer,
    founded_year integer,
    created_at   timestamp with time zone DEFAULT now() NOT NULL,
    updated_at   timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (entity_id),
    CONSTRAINT companies_entity_fk FOREIGN KEY (entity_id)
        REFERENCES public.entities(id) ON DELETE CASCADE
);

-- Seed a couple shared company + person entities so the surface renders end to end before
-- ingest wires real data. Fixed uuids so the extension/external rows can reference them.
INSERT INTO public.entities (id, scope, kind, canonical_name, normalized_key, trust_tier, gist, data_points) VALUES
    ('c0000000-0000-4000-a000-0000000000a1', 'shared', 'company', 'Apple Inc.', 'apple inc', 'believed',
     'Designs and sells consumer electronics, software, and services.',
     '[{"name":"conviction","type":"select","value":"High"},{"name":"catalyst","type":"date","value":"2026-08-27"},{"name":"thesis_status","type":"select","value":"Researching"}]'::jsonb),
    ('c0000000-0000-4000-a000-0000000000a2', 'shared', 'company', 'NVIDIA Corporation', 'nvidia corporation', 'believed',
     'Designs GPUs and accelerated-computing platforms for AI and graphics.',
     '[{"name":"conviction","type":"select","value":"High"},{"name":"thesis_status","type":"select","value":"Validated"}]'::jsonb),
    ('c0000000-0000-4000-a000-0000000000b1', 'shared', 'person', 'Tim Cook', 'tim cook', 'believed',
     'Chief Executive Officer of Apple.',
     '[{"name":"role","type":"text","value":"CEO, Apple"},{"name":"since","type":"date","value":"2011-08-24"}]'::jsonb),
    ('c0000000-0000-4000-a000-0000000000b2', 'shared', 'person', 'Jensen Huang', 'jensen huang', 'believed',
     'Co-founder and CEO of NVIDIA.',
     '[{"name":"role","type":"text","value":"Co-founder & CEO, NVIDIA"}]'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.companies (entity_id, sector, industry, description, website, country, employees, founded_year) VALUES
    ('c0000000-0000-4000-a000-0000000000a1', 'Technology', 'Consumer Electronics',
     'Apple designs, manufactures and markets smartphones, personal computers, tablets, wearables and accessories, and sells a variety of related services.',
     'https://www.apple.com', 'United States', 164000, 1976),
    ('c0000000-0000-4000-a000-0000000000a2', 'Technology', 'Semiconductors',
     'NVIDIA provides graphics, compute and networking solutions; its GPUs power gaming, professional visualization, data center and automotive markets.',
     'https://www.nvidia.com', 'United States', 29600, 1993)
ON CONFLICT (entity_id) DO NOTHING;

INSERT INTO public.entity_external_ids (entity_id, system, value) VALUES
    ('c0000000-0000-4000-a000-0000000000a1', 'cik', '0000320193'),
    ('c0000000-0000-4000-a000-0000000000a2', 'cik', '0001045810')
ON CONFLICT (system, value) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS public.companies;
DROP TABLE IF EXISTS public.entity_external_ids;
ALTER TABLE public.entities DROP COLUMN IF EXISTS data_points;
