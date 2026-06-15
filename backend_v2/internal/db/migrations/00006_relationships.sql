-- +goose Up
-- relationships is an ADDITIVE, polymorphic edge layer that connects the two
-- worlds (workspace: artifacts · ingestion: records, insights) WITHOUT unifying
-- their tables — the "bridge-first" path from DATA_MODEL.md (D1), deliberately
-- NOT the contested table-unification. Edges are typed; endpoints are stored as
-- (kind discriminator + text id), since artifact/record ids are text and insight
-- ids are uuids — so no single FK spans them. Referential integrity for the
-- endpoints is enforced at the service layer on create. Fully reversible.
CREATE TABLE public.relationships (
    id          uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id     uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    src_kind    text NOT NULL,
    src_id      text NOT NULL,
    dst_kind    text NOT NULL,
    dst_id      text NOT NULL,
    rel_type    text NOT NULL,
    metadata    jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at  timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT relationships_src_kind_chk CHECK (src_kind IN ('artifact', 'record', 'insight')),
    CONSTRAINT relationships_dst_kind_chk CHECK (dst_kind IN ('artifact', 'record', 'insight')),
    CONSTRAINT relationships_type_chk CHECK (rel_type IN ('cites', 'supports', 'contradicts', 'about', 'derived_from'))
);

-- one edge per (owner, src, dst, type); a re-assert is idempotent
CREATE UNIQUE INDEX uq_relationships_edge ON public.relationships (user_id, src_kind, src_id, dst_kind, dst_id, rel_type);
-- traverse outward and inward from a node
CREATE INDEX idx_relationships_src ON public.relationships (user_id, src_kind, src_id);
CREATE INDEX idx_relationships_dst ON public.relationships (user_id, dst_kind, dst_id);

-- +goose Down
DROP TABLE public.relationships;
