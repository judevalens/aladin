-- +goose Up
-- P1 entity hardening (plan: ~/.claude/plans/entity-hardening.md).
--
-- 1) Canonical-name-as-alias invariant: every entity gets an alias row for its own
--    canonical name, so the alias-aware picker search (P1.1) matches every entity
--    uniformly through entity_aliases. New create paths seed this row themselves;
--    this backfills the pre-existing registry.
-- 2) Retire kind='unknown' → 'other' (the generic typed-kind set is
--    person | org | concept | location | other).
-- 3) Trigram index on entity_aliases.normalized for the picker's prefix/similarity
--    matching (pg_trgm is enabled in 00001).
--
-- Placeholder entities (P1.2 — minted by the editor's @ create path when nothing
-- matches) are marked trust_tier='placeholder'. trust_tier is free text, so no
-- constraint change is needed; this comment documents the canonical value. The
-- background judge later resolves placeholders: merge into an existing entity, or
-- promote in place (trust_tier → 'believed', kind backfilled).
--
-- Deliberately NO unique index on entities.normalized_key or on alias surfaces
-- across entities: sense-splitting (one surface, several senses) is a designed
-- feature of the resolver — global uniqueness would break it. Create-path dedup
-- stays best-effort SELECT-then-INSERT (acceptable for the manual path).

INSERT INTO entity_aliases (entity_id, surface, normalized, kind, source)
SELECT e.id, e.canonical_name, e.normalized_key, e.kind, 'canonical'
  FROM entities e
 WHERE e.normalized_key <> ''
ON CONFLICT (entity_id, normalized) DO NOTHING;

UPDATE entities SET kind = 'other' WHERE kind = 'unknown';
UPDATE entity_aliases SET kind = 'other' WHERE kind = 'unknown';
ALTER TABLE entities ALTER COLUMN kind SET DEFAULT 'other';
ALTER TABLE entity_aliases ALTER COLUMN kind SET DEFAULT 'other';

CREATE INDEX IF NOT EXISTS entity_aliases_norm_trgm_idx
    ON entity_aliases USING gin (normalized gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS entity_aliases_norm_trgm_idx;
ALTER TABLE entity_aliases ALTER COLUMN kind SET DEFAULT 'unknown';
ALTER TABLE entities ALTER COLUMN kind SET DEFAULT 'unknown';
-- Data backfills (canonical aliases, kind renames) are intentionally not reverted:
-- they are additive/harmless under the old code paths.
