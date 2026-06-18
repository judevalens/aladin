// Package claims implements the claim layer (C0): contestable, entity-grounded
// propositions extracted from records, stored as canonical claims + their subjects + the
// evidence (which source asserts them). Paraphrase-aware resolution, the argument edges,
// the authored side, and the contradiction surface are later phases.
package claims

import (
	"context"
	"log/slog"
	"strings"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

const (
	// claimResolveMinCosine is the embedding floor for a claim to be a resolution
	// candidate (claims are paraphrase-heavy, so this is the primary signal).
	claimResolveMinCosine = 0.8
	claimCandidateLimit   = 5
)

// Service runs claim extraction + resolution. The embedder + adjudicator are optional
// (C1): without them it falls back to C0 behavior (exact-text dedup + create).
type Service struct {
	store       db.ClaimRepository
	extractor   llm.ClaimExtractor
	embedder    llm.Embedder
	adjudicator llm.ClaimAdjudicator
}

func NewService(store db.ClaimRepository, extractor llm.ClaimExtractor) *Service {
	return &Service{store: store, extractor: extractor}
}

// WithEmbedder enables embedding-based claim resolution (C1).
func (s *Service) WithEmbedder(e llm.Embedder) *Service {
	s.embedder = e
	return s
}

// WithAdjudicator enables polarity-aware claim adjudication: a negation resolves to the
// canonical claim as a deny mention; a related claim gets a proposed argument edge.
func (s *Service) WithAdjudicator(a llm.ClaimAdjudicator) *Service {
	s.adjudicator = a
	return s
}

// RecordInput is the extraction context for one enriched record.
type RecordInput struct {
	RecordID  string
	Summary   string
	KeyClaims []string
	Entities  []db.EntityRef // the record's resolved entities (the grounding set)
}

// ExtractFromRecord runs the contestability-gated extraction: lift contestable,
// entity-grounded claims and store each as a canonical claim + subject edges + an
// asserting mention. Two gates, both enforced here so the LLM can't flood the layer:
// (1) the model marks it contestable, and (2) it's about ≥1 of the record's resolved
// entities. C0 dedup is exact-text only (paraphrase resolution is C1). A nil/erroring
// extractor is a no-op — resolution-style graceful degrade; the pipeline never fails on it.
// Returns the number of claims stored.
func (s *Service) ExtractFromRecord(ctx context.Context, in RecordInput) (int, error) {
	if s.extractor == nil || len(in.KeyClaims) == 0 {
		return 0, nil
	}

	entByName := make(map[string]string, len(in.Entities))
	llmEntities := make([]llm.ClaimEntity, 0, len(in.Entities))
	for _, e := range in.Entities {
		entByName[strings.ToLower(strings.TrimSpace(e.Name))] = e.ID
		llmEntities = append(llmEntities, llm.ClaimEntity{ID: e.ID, Name: e.Name})
	}
	// No grounding entities → nothing can pass the entity-grounded gate.
	if len(entByName) == 0 {
		return 0, nil
	}

	extracted, err := s.extractor.ExtractClaims(ctx, llm.ClaimExtractionInput{
		Summary:   in.Summary,
		KeyClaims: in.KeyClaims,
		Entities:  llmEntities,
	})
	if err != nil {
		slog.Warn("claims: extract failed, skipping record", "component", "claims", "record_id", in.RecordID, "err", err)
		return 0, nil
	}

	stored := 0
	for _, c := range extracted {
		text := strings.TrimSpace(c.Text)
		if !c.Contestable || text == "" {
			continue // contestability gate
		}
		subjectIDs := matchSubjects(c.SubjectNames, entByName)
		if len(subjectIDs) == 0 {
			continue // entity-grounded gate
		}

		claimID, stance, err := s.resolveClaim(ctx, "shared", "", text, normalizePolarity(c.Polarity), subjectIDs, in.RecordID)
		if err != nil {
			return stored, err
		}
		for _, eid := range subjectIDs {
			if err := s.store.AddClaimSubject(ctx, claimID, eid); err != nil {
				return stored, err
			}
		}
		if err := s.store.AddClaimMention(ctx, db.ClaimMentionParams{
			ClaimID:    claimID,
			SourceKind: "record",
			SourceID:   in.RecordID,
			Stance:     stance,
			Resolver:   "extract",
		}); err != nil {
			return stored, err
		}
		stored++
	}
	return stored, nil
}

// resolveClaim maps a claim to a canonical claim id + the source's stance on it. Order:
// exact text → embedding+adjudicator (C1) → create new. A "negation" verdict is the
// crux of the contradiction feature: it resolves to the SAME canonical claim but the
// source's stance is 'deny', so support/contradict is just assert/deny mentions on one
// proposition. "related" creates a new claim + a proposed argument edge.
func (s *Service) resolveClaim(ctx context.Context, scope, owner, text, polarity string, subjectIDs []string, sourceID string) (string, string, error) {
	if id, found, err := s.store.FindClaimByText(ctx, scope, owner, text); err != nil {
		return "", "", err
	} else if found {
		return id, "assert", nil
	}

	var vec []float32
	if s.embedder != nil {
		if v, err := s.embedder.Embed(ctx, text); err != nil {
			slog.Warn("claims: embed failed, skipping vector resolution", "component", "claims", "err", err)
		} else {
			vec = v
		}
	}

	if len(vec) > 0 && s.adjudicator != nil {
		cands, err := s.store.FindClaimCandidates(ctx, scope, owner, subjectIDs, vec, claimResolveMinCosine, claimCandidateLimit)
		if err != nil {
			return "", "", err
		}
		if len(cands) > 0 {
			best := cands[0]
			rel, jerr := s.adjudicator.JudgeClaims(ctx, llm.ClaimAdjudicationInput{A: text, B: best.CanonicalText})
			switch {
			case jerr != nil:
				slog.Warn("claims: adjudicator failed, treating as new claim", "component", "claims", "err", jerr)
			case rel.Relation == "same":
				return best.ID, "assert", nil
			case rel.Relation == "negation":
				return best.ID, "deny", nil // the contradiction mechanism
			case rel.Relation == "related":
				id, err := s.createClaim(ctx, scope, owner, text, polarity, sourceID, vec)
				if err != nil {
					return "", "", err
				}
				edge := rel.EdgeType
				if edge == "" || edge == "none" {
					edge = "qualifies"
				}
				if _, err := s.store.AddClaimEdge(ctx, db.ClaimEdgeParams{
					FromClaimID: id, ToClaimID: best.ID, Type: edge, Confidence: rel.Confidence, Method: "llm",
				}); err != nil {
					return "", "", err
				}
				return id, "assert", nil
			}
		}
	}

	id, err := s.createClaim(ctx, scope, owner, text, polarity, sourceID, vec)
	return id, "assert", err
}

func (s *Service) createClaim(ctx context.Context, scope, owner, text, polarity, sourceID string, vec []float32) (string, error) {
	id, err := s.store.CreateClaim(ctx, db.CreateClaimParams{
		Scope: scope, OwnerUserID: owner, CanonicalText: text, Polarity: polarity, FirstSourceID: sourceID,
	})
	if err != nil {
		return "", err
	}
	if len(vec) > 0 {
		if err := s.store.SetClaimEmbedding(ctx, id, vec); err != nil {
			return "", err
		}
	}
	return id, nil
}

// matchSubjects maps the model's subject names back to the record's resolved entity ids
// (case-insensitive, deduped). A name the model invented that isn't in the grounding set
// is dropped — that's the grounding guarantee.
func matchSubjects(names []string, byName map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if id, ok := byName[strings.ToLower(strings.TrimSpace(n))]; ok && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func normalizePolarity(p string) string {
	switch p {
	case "assert", "deny", "neutral":
		return p
	default:
		return "assert"
	}
}
