package entities

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

const (
	// fuzzyProposeMinSim is the trigram-similarity floor below which a near-match is
	// treated as unrelated (no proposal). R1 never *auto-merges* a fuzzy match — even a
	// near-identical candidate is queued as a proposed merge for review (decision D7d).
	fuzzyProposeMinSim  = 0.45
	fuzzyCandidateLimit = 5

	// senseSplitThreshold (R2/§5.7): when an exact normalized-key entity already exists
	// and a context-embedding is available, a new mention whose context cosine to every
	// existing sense is below this is treated as a *different sense* (same name, same
	// kind, different entity) and split off. Tune with real data.
	senseSplitThreshold = 0.5
	// vectorProposeMinCosine: on an exact-key miss, an embedding neighbor at least this
	// similar is queued as a proposed merge (a semantic signal trigram blocking misses).
	vectorProposeMinCosine = 0.75
)

// Resolver maps free-text entity mentions to canonical entity ids. R0 is the
// foundation: normalize → exact-key lookup → resolve-or-create (idempotent alias +
// mention). R1 adds fuzzy near-match detection: on an exact miss it still creates the
// entity but queues a *proposed merge* against the best trigram candidate (respecting
// rejected-pair negative evidence). Auto-merge, embeddings, the LLM adjudicator, the
// tenant tier, and same-name/same-kind context splitting are later phases.
type Resolver struct {
	store       db.EntityRepository
	adjudicator llm.EntityAdjudicator // optional (R2); nil → deterministic behavior
	embedder    llm.Embedder          // optional (R2); nil → no vector signal / no sense split
}

func NewResolver(store db.EntityRepository) *Resolver {
	return &Resolver{store: store}
}

// WithAdjudicator enables the R2 LLM adjudication of fuzzy candidates. When set, a
// "different" verdict suppresses the proposal as negative evidence and "same" raises
// its confidence; a nil adjudicator (the default) keeps pure trigram proposing.
func (r *Resolver) WithAdjudicator(a llm.EntityAdjudicator) *Resolver {
	r.adjudicator = a
	return r
}

// WithEmbedder enables R2 context-embeddings: new entities are embedded, vector
// near-matches augment trigram candidates, and exact-key mentions whose context
// diverges are split into a new sense (§5.7). A nil embedder (the default) keeps pure
// string behavior. Embedding errors degrade gracefully — resolution never fails on it.
func (r *Resolver) WithEmbedder(e llm.Embedder) *Resolver {
	r.embedder = e
	return r
}

// Mention is one occurrence of an entity surface form in a record.
type Mention struct {
	Surface        string
	Kind           string // "" → "other" (generic typed kinds: person|org|concept|location|other)
	RecordID       string
	SourceRevision int64
	ContextHint    string // optional surrounding context (e.g. record summary) for the context-embedding
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
		kind = "other"
	}
	norm := Normalize(surface)
	if norm == "" {
		return "", nil
	}

	// R2: a context-embedding for vector matching + sense routing. nil/error → degrade
	// to pure string behavior (resolution never fails on the embedder).
	ctxVec, hasVec := r.embedContext(ctx, surface, m.ContextHint)

	cands, err := r.store.FindSharedByKey(ctx, kind, norm)
	if err != nil {
		return "", fmt.Errorf("entities: find by key: %w", err)
	}

	var (
		id       string
		resolver = "alias"
	)
	switch {
	case len(cands) == 0:
		// Unseen key → create, store its embedding, and queue a proposed merge against
		// the best fuzzy/vector near-match — never auto-merge (D7d). The just-created key
		// is excluded from the candidate queries (it's an exact, not a near, match).
		id, err = r.store.CreateSharedEntity(ctx, db.CreateEntityParams{
			Kind: kind, CanonicalName: surface, NormalizedKey: norm, FirstRecordID: m.RecordID,
		})
		if err != nil {
			return "", fmt.Errorf("entities: create: %w", err)
		}
		resolver = "new"
		if hasVec {
			if err := r.store.SetEntityEmbedding(ctx, id, ctxVec); err != nil {
				return "", fmt.Errorf("entities: set embedding: %w", err)
			}
		}
		if err := r.proposeFuzzyMerge(ctx, id, kind, norm, surface, ctxVec, hasVec); err != nil {
			return "", err
		}
	case hasVec:
		// Exact key exists AND context is comparable → route to the best-matching sense,
		// or split off a new sense if the context diverges from all of them (§5.7).
		best, cos, comparable := r.bestSenseByVector(ctx, cands, ctxVec)
		if !comparable || cos >= senseSplitThreshold {
			id = best
		} else {
			id, err = r.store.CreateSharedEntity(ctx, db.CreateEntityParams{
				Kind: kind, CanonicalName: surface, NormalizedKey: norm, FirstRecordID: m.RecordID,
			})
			if err != nil {
				return "", fmt.Errorf("entities: create (sense split): %w", err)
			}
			if err := r.store.SetEntityEmbedding(ctx, id, ctxVec); err != nil {
				return "", fmt.Errorf("entities: set embedding: %w", err)
			}
			resolver = "split"
		}
	default:
		// Exact key exists, no vectors → resolve to the oldest (deterministic R0 behavior).
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

// ResolveTenant resolves a mention into a tenant's overlay (Tier 1), per the §5.0
// cascade: probe the tenant's own entities first, else bind to a matching shared
// (Tier 0) entity, else create a new tenant-local entity. This is the authored-content
// resolve path — built and tested, but not yet wired into the public-ingestion
// pipeline (which resolves into Tier 0 via Resolve).
func (r *Resolver) ResolveTenant(ctx context.Context, ownerUserID string, m Mention) (string, error) {
	surface := strings.TrimSpace(m.Surface)
	if surface == "" || ownerUserID == "" {
		return "", nil
	}
	kind := m.Kind
	if kind == "" {
		kind = "other"
	}
	norm := Normalize(surface)
	if norm == "" {
		return "", nil
	}

	resolver := "alias"
	var id string
	tcands, err := r.store.FindTenantByKey(ctx, ownerUserID, kind, norm)
	if err != nil {
		return "", fmt.Errorf("entities: find tenant key: %w", err)
	}
	if len(tcands) >= 1 {
		id = tcands[0].ID
	} else {
		// No tenant overlay yet — bind to the shared canonical if one exists, else go private.
		sharedID := ""
		scands, err := r.store.FindSharedByKey(ctx, kind, norm)
		if err != nil {
			return "", fmt.Errorf("entities: find shared key: %w", err)
		}
		if len(scands) >= 1 {
			sharedID = scands[0].ID
		}
		id, err = r.store.CreateTenantEntity(ctx, db.CreateTenantEntityParams{
			OwnerUserID:    ownerUserID,
			SharedEntityID: sharedID,
			Kind:           kind,
			CanonicalName:  surface,
			NormalizedKey:  norm,
			FirstRecordID:  m.RecordID,
		})
		if err != nil {
			return "", fmt.Errorf("entities: create tenant entity: %w", err)
		}
		if sharedID != "" {
			resolver = "bind"
		} else {
			resolver = "new"
		}
	}

	if err := r.store.AddAlias(ctx, db.AliasParams{
		EntityID:   id,
		Surface:    surface,
		Normalized: norm,
		Kind:       kind,
		Source:     "authored",
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
func (r *Resolver) proposeFuzzyMerge(ctx context.Context, newID, kind, norm, surface string, ctxVec []float32, hasVec bool) error {
	scored, err := r.store.FindSharedCandidates(ctx, kind, norm, fuzzyProposeMinSim, fuzzyCandidateLimit)
	if err != nil {
		return fmt.Errorf("entities: find candidates: %w", err)
	}
	if hasVec {
		vcands, verr := r.store.FindSharedCandidatesByVector(ctx, kind, norm, ctxVec, vectorProposeMinCosine, fuzzyCandidateLimit)
		if verr != nil {
			return fmt.Errorf("entities: find vector candidates: %w", verr)
		}
		scored = mergeCandidates(scored, vcands)
	}
	if len(scored) == 0 {
		return nil
	}
	best := scored[0]
	method := "trigram"
	confidence := best.Similarity

	// R2: let the adjudicator settle the ambiguous match. "different" becomes negative
	// evidence (no proposal); "same" raises confidence. An adjudicator error degrades
	// gracefully to a plain trigram proposal — resolution must never fail on the LLM.
	if r.adjudicator != nil {
		verdict, jerr := r.adjudicator.JudgeSameEntity(ctx, llm.EntityAdjudicationInput{
			Kind: kind,
			A:    surface,
			B:    best.CanonicalName,
		})
		switch {
		case jerr != nil:
			slog.Warn("entities: adjudicator failed, falling back to trigram", "component", "entities", "err", jerr)
		case verdict.Verdict == "different":
			if err := r.store.RecordDistinct(ctx, newID, best.ID, "llm", verdict.Reason); err != nil {
				return fmt.Errorf("entities: record distinct: %w", err)
			}
			return nil
		case verdict.Verdict == "same":
			method = "llm"
			confidence = verdict.Confidence
		default: // "uncertain" or anything unexpected → still propose for human review
			method = "trigram+llm"
		}
	}

	if _, err := r.store.ProposeMerge(ctx, db.ProposeMergeParams{
		FromEntityID: newID,
		IntoEntityID: best.ID,
		Confidence:   confidence,
		Method:       method,
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

// embedContext produces the context-embedding for a mention (surface + optional hint).
// Returns (nil, false) when no embedder is configured or the call fails — the caller
// then degrades to pure string behavior.
func (r *Resolver) embedContext(ctx context.Context, surface, hint string) ([]float32, bool) {
	if r.embedder == nil {
		return nil, false
	}
	text := surface
	if hint != "" {
		text = surface + " — " + hint
	}
	vec, err := r.embedder.Embed(ctx, text)
	if err != nil {
		slog.Warn("entities: embed failed, skipping vector signal", "component", "entities", "err", err)
		return nil, false
	}
	if len(vec) == 0 {
		return nil, false
	}
	return vec, true
}

// bestSenseByVector picks the exact-key candidate whose stored embedding is most
// similar to vec. comparable is false when no candidate has an embedding (then the
// caller resolves to the oldest rather than splitting blindly).
func (r *Resolver) bestSenseByVector(ctx context.Context, cands []db.EntityCandidate, vec []float32) (id string, cos float64, comparable bool) {
	best := -2.0
	bestID := ""
	for _, c := range cands {
		ev, has, err := r.store.GetEntityEmbedding(ctx, c.ID)
		if err != nil || !has {
			continue
		}
		if cs := cosine(vec, ev); cs > best {
			best = cs
			bestID = c.ID
		}
	}
	if bestID == "" {
		return cands[0].ID, 0, false
	}
	return bestID, best, true
}

// mergeCandidates unions trigram and vector candidates, deduped by id keeping the higher
// score, best first. Trigram similarity and vector cosine are both ~[0,1]; max is a fine
// heuristic for "the single best candidate to propose."
func mergeCandidates(a, b []db.ScoredCandidate) []db.ScoredCandidate {
	byID := make(map[string]db.ScoredCandidate, len(a)+len(b))
	add := func(c db.ScoredCandidate) {
		if ex, ok := byID[c.ID]; !ok || c.Similarity > ex.Similarity {
			byID[c.ID] = c
		}
	}
	for _, c := range a {
		add(c)
	}
	for _, c := range b {
		add(c)
	}
	out := make([]db.ScoredCandidate, 0, len(byID))
	for _, c := range byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Similarity > out[j].Similarity })
	return out
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
