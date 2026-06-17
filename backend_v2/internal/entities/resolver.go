package entities

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/db"
)

const (
	// fuzzyProposeMinSim is the trigram-similarity floor below which a near-match is
	// treated as unrelated (no proposal). R1 never *auto-merges* a fuzzy match — even a
	// near-identical candidate is queued as a proposed merge for review (decision D7d).
	fuzzyProposeMinSim = 0.45
	fuzzyCandidateLimit = 5
)

// Resolver maps free-text entity mentions to canonical entity ids. R0 is the
// foundation: normalize → exact-key lookup → resolve-or-create (idempotent alias +
// mention). R1 adds fuzzy near-match detection: on an exact miss it still creates the
// entity but queues a *proposed merge* against the best trigram candidate (respecting
// rejected-pair negative evidence). Auto-merge, embeddings, the LLM adjudicator, the
// tenant tier, and same-name/same-kind context splitting are later phases.
type Resolver struct {
	store db.EntityRepository
}

func NewResolver(store db.EntityRepository) *Resolver {
	return &Resolver{store: store}
}

// Mention is one occurrence of an entity surface form in a record.
type Mention struct {
	Surface        string
	Kind           string // "" → "unknown" in R0 (this branch's enrichment is untyped)
	RecordID       string
	SourceRevision int64
}

// Resolve resolves a single mention to a shared-tier entity id, creating the entity
// when the normalized key is unseen. It always records an alias and a mention.
// Empty / punctuation-only surfaces are skipped (returns id == "").
func (r *Resolver) Resolve(ctx context.Context, m Mention) (string, error) {
	surface := strings.TrimSpace(m.Surface)
	if surface == "" {
		return "", nil
	}
	kind := m.Kind
	if kind == "" {
		kind = "unknown"
	}
	norm := Normalize(surface)
	if norm == "" {
		return "", nil
	}

	cands, err := r.store.FindSharedByKey(ctx, kind, norm)
	if err != nil {
		return "", fmt.Errorf("entities: find by key: %w", err)
	}

	var (
		id       string
		resolver = "alias"
	)
	if len(cands) == 0 {
		id, err = r.store.CreateSharedEntity(ctx, db.CreateEntityParams{
			Kind:          kind,
			CanonicalName: surface,
			NormalizedKey: norm,
			FirstRecordID: m.RecordID,
		})
		if err != nil {
			return "", fmt.Errorf("entities: create: %w", err)
		}
		resolver = "new"

		// R1: no exact match, but a fuzzy near-match may be the same entity. Queue the
		// best candidate as a proposed merge for review — never auto-merge (D7d). The
		// just-created key is excluded by the candidate query (it matches exact, not fuzzy).
		if err := r.proposeFuzzyMerge(ctx, id, kind, norm, surface); err != nil {
			return "", err
		}
	} else {
		// One match → resolve to it. ≥2 (a key that became ambiguous via a later
		// phase's split) → pick the oldest deterministically; true sense
		// disambiguation is R1+ (spec §5.7). FindSharedByKey orders oldest-first.
		id = cands[0].ID
	}

	if err := r.store.AddAlias(ctx, db.AliasParams{
		EntityID:   id,
		Surface:    surface,
		Normalized: norm,
		Kind:       kind,
		Source:     "enrichment",
	}); err != nil {
		return "", fmt.Errorf("entities: add alias: %w", err)
	}
	if err := r.store.AddMention(ctx, db.MentionParams{
		RecordID:       m.RecordID,
		EntityID:       id,
		Surface:        surface,
		Kind:           kind,
		Resolver:       resolver,
		SourceRevision: m.SourceRevision,
	}); err != nil {
		return "", fmt.Errorf("entities: add mention: %w", err)
	}
	return id, nil
}

// proposeFuzzyMerge queues a proposed merge between the newly-created entity newID and
// the best trigram near-match (if any above the floor). ProposeMerge itself is a no-op
// when the pair already has a row in any status, so a previously rejected pair (negative
// evidence) is never re-proposed.
func (r *Resolver) proposeFuzzyMerge(ctx context.Context, newID, kind, norm, surface string) error {
	scored, err := r.store.FindSharedCandidates(ctx, kind, norm, fuzzyProposeMinSim, fuzzyCandidateLimit)
	if err != nil {
		return fmt.Errorf("entities: find candidates: %w", err)
	}
	if len(scored) == 0 {
		return nil
	}
	best := scored[0]
	if _, err := r.store.ProposeMerge(ctx, db.ProposeMergeParams{
		FromEntityID: newID,
		IntoEntityID: best.ID,
		Confidence:   best.Similarity,
		Method:       "trigram",
		Evidence: map[string]any{
			"surface":    surface,
			"candidate":  best.CanonicalName,
			"similarity": best.Similarity,
		},
	}); err != nil {
		return fmt.Errorf("entities: propose merge: %w", err)
	}
	return nil
}
