-- +goose Up
-- Remove the claim layer. The claim/"Signals" concept is gone from the product: claims,
-- their subjects/mentions/edges, the resolve_claims pipeline stage, the authored-page
-- extractor (pageingest), the Signals surface, and the "signal" sync kind. The word
-- "signal" is freed for its trading meaning (a strategy's output on a bar).
--
-- The entity layer STAYS — it is what the trading entity model (instruments, companies)
-- is built on, and Neo4j keeps projecting it (entities + MERGED_INTO + RELATED_TO). What
-- goes is everything claim-anchored: the ABOUT / SUPPORTS / CONTRADICTS / QUALIFIES /
-- DIVERGES_FROM edges each had a claim on at least one end.
--
-- Forward-only. 00011 (claim_layer), 00012 (discourse — which stays) and 00015
-- (claims_seq) are applied migrations and are left untouched.

-- `#` cross-references pointing at claims: the targets are about to disappear, and the
-- picker no longer offers the kind. Drop the rows, then tighten the constraint to match
-- the code (internal/service/artifact_ref.go validRefKinds, internal/blocknote/refs.go).
DELETE FROM public.artifact_refs WHERE target_kind = 'claim';

ALTER TABLE public.artifact_refs DROP CONSTRAINT IF EXISTS artifact_refs_kind_check;
ALTER TABLE public.artifact_refs
    ADD CONSTRAINT artifact_refs_kind_check CHECK (target_kind IN ('page', 'shard'));

-- Child tables first (every FK points back at claims).
DROP TABLE IF EXISTS public.claim_edges;
DROP TABLE IF EXISTS public.claim_mentions;
DROP TABLE IF EXISTS public.claim_subjects;
DROP TABLE IF EXISTS public.claims;

-- +goose Down
-- IRREVERSIBLE by design: this migration deletes data, and the code that read these tables
-- was removed in the same change. Rolling back only reopens the artifact_refs constraint so
-- the pre-removal check is restored; the claim tables are NOT recreated. To genuinely
-- revert, roll back to the commit before the removal and re-run 00011/00015 from scratch.
ALTER TABLE public.artifact_refs DROP CONSTRAINT IF EXISTS artifact_refs_kind_check;
ALTER TABLE public.artifact_refs
    ADD CONSTRAINT artifact_refs_kind_check CHECK (target_kind IN ('claim', 'page', 'shard'));
