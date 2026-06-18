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

// Service runs discovered-side claim extraction.
type Service struct {
	store     db.ClaimRepository
	extractor llm.ClaimExtractor
}

func NewService(store db.ClaimRepository, extractor llm.ClaimExtractor) *Service {
	return &Service{store: store, extractor: extractor}
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

		claimID, found, err := s.store.FindClaimByText(ctx, "shared", "", text)
		if err != nil {
			return stored, err
		}
		if !found {
			claimID, err = s.store.CreateClaim(ctx, db.CreateClaimParams{
				Scope:         "shared",
				CanonicalText: text,
				Polarity:      normalizePolarity(c.Polarity),
				FirstSourceID: in.RecordID,
			})
			if err != nil {
				return stored, err
			}
		}
		for _, eid := range subjectIDs {
			if err := s.store.AddClaimSubject(ctx, claimID, eid); err != nil {
				return stored, err
			}
		}
		// C0: the extracting source ASSERTS the claim as stated (its polarity carries the
		// affirmative/negative form). C1/C3 reconcile negations into deny-stance + contradicts.
		if err := s.store.AddClaimMention(ctx, db.ClaimMentionParams{
			ClaimID:    claimID,
			SourceKind: "record",
			SourceID:   in.RecordID,
			Stance:     "assert",
			Resolver:   "extract",
		}); err != nil {
			return stored, err
		}
		stored++
	}
	return stored, nil
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
