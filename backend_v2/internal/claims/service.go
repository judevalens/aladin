// Package claims implements the claim layer (C0): contestable, entity-grounded
// propositions extracted from records, stored as canonical claims + their subjects + the
// evidence (which source asserts them). Paraphrase-aware resolution, the argument edges,
// the authored side, and the contradiction surface are later phases.
package claims

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/llm"
)

const (
	// claimResolveMinCosine is the embedding floor for a claim to be a vector-channel
	// resolution candidate (claims are paraphrase-heavy, so this is the primary signal).
	claimResolveMinCosine = 0.8
	claimCandidateLimit   = 5  // vector channel (pgvector) top-N
	claimGraphLimit       = 10 // graph channel (Neo4j) top-N
	// claimAdjudicateMax bounds the LLM adjudications per new claim: we judge the new claim
	// against at most this many RRF-fused candidates (early-exit on same/negation), NEVER
	// against every existing claim. This is the cost ceiling of resolution.
	claimAdjudicateMax = 5
	rrfK               = 60 // Reciprocal Rank Fusion constant
)

// GraphClaimSource is the graph channel of hybrid claim retrieval: claims reachable from a set
// of subject entities through the connection graph (the subject itself + RELATED_TO/MERGED_INTO
// neighbours). Optional — nil falls back to pure pgvector retrieval. *graph.Projector satisfies it.
type GraphClaimSource interface {
	ClaimCandidates(ctx context.Context, subjectEntityIDs []string, limit int) ([]graph.ClaimCandidate, error)
}

// Service runs claim extraction + resolution. The embedder + adjudicator are optional
// (C1): without them it falls back to C0 behavior (exact-text dedup + create). The graph
// channel (graphCands) is also optional: without it, retrieval is pgvector-only.
type Service struct {
	store       db.ClaimRepository
	extractor   llm.ClaimExtractor
	embedder    llm.Embedder
	adjudicator llm.ClaimAdjudicator
	graphCands  GraphClaimSource
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

// WithGraphCandidates enables the graph retrieval channel (Neo4j). Without it, candidate
// retrieval is pgvector-only — exactly the prior behaviour.
func (s *Service) WithGraphCandidates(g GraphClaimSource) *Service {
	s.graphCands = g
	return s
}

// RecordInput is the extraction context for one enriched record.
type RecordInput struct {
	RecordID  string
	Summary   string
	KeyClaims []string
	Entities  []db.EntityRef // the record's resolved entities (the grounding set)
}

// claimSink is the scope/trust/source context a batch of extracted claims lands in.
type claimSink struct {
	Scope      string
	Owner      string
	Trust      string
	SourceKind string
	SourceID   string
}

// AuthoredInput is a user's authored page/thesis: its claims become tenant-scoped,
// verified claims (C2 — the authored side of the bridge). The caller resolves the page's
// entities (tenant tier) and passes them as the grounding set.
type AuthoredInput struct {
	OwnerID    string
	ArtifactID string
	Summary    string
	KeyClaims  []string
	Entities   []db.EntityRef
}

// ExtractFromRecord runs contestability-gated extraction over a discovered record into
// shared, believed claims.
func (s *Service) ExtractFromRecord(ctx context.Context, in RecordInput) (int, error) {
	return s.extract(ctx,
		claimSink{Scope: "shared", Trust: "believed", SourceKind: "record", SourceID: in.RecordID},
		in.Summary, in.KeyClaims, in.Entities)
}

// ExtractFromAuthored extracts a user's theses from authored content into tenant,
// verified claims — the theses the contradiction surface defends.
func (s *Service) ExtractFromAuthored(ctx context.Context, in AuthoredInput) (int, error) {
	if in.OwnerID == "" {
		return 0, nil
	}
	return s.extract(ctx,
		claimSink{Scope: "tenant", Owner: in.OwnerID, Trust: "verified", SourceKind: "artifact", SourceID: in.ArtifactID},
		in.Summary, in.KeyClaims, in.Entities)
}

// extract is the shared extraction core. Two gates (contestable + entity-grounded) keep
// the layer clean; a nil/erroring extractor is a no-op (graceful degrade). Returns the
// number of claims stored.
func (s *Service) extract(ctx context.Context, sink claimSink, summary string, keyClaims []string, entities []db.EntityRef) (int, error) {
	if s.extractor == nil || len(keyClaims) == 0 {
		return 0, nil
	}

	entByName := make(map[string]string, len(entities))
	llmEntities := make([]llm.ClaimEntity, 0, len(entities))
	for _, e := range entities {
		entByName[strings.ToLower(strings.TrimSpace(e.Name))] = e.ID
		llmEntities = append(llmEntities, llm.ClaimEntity{ID: e.ID, Name: e.Name})
	}
	if len(entByName) == 0 {
		return 0, nil // nothing can pass the entity-grounded gate
	}

	extracted, err := s.extractor.ExtractClaims(ctx, llm.ClaimExtractionInput{
		Summary:   summary,
		KeyClaims: keyClaims,
		Entities:  llmEntities,
	})
	if err != nil {
		slog.Warn("claims: extract failed, skipping source", "component", "claims", "source_id", sink.SourceID, "err", err)
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

		claimID, stance, err := s.resolveClaim(ctx, sink.Scope, sink.Owner, sink.Trust, text, normalizePolarity(c.Polarity), subjectIDs, sink.SourceID)
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
			SourceKind: sink.SourceKind,
			SourceID:   sink.SourceID,
			Stance:     stance,
			Resolver:   "extract",
		}); err != nil {
			return stored, err
		}
		stored++
	}
	return stored, nil
}

// GetContradictions returns the support/contradict tally for a thesis claim — the C4
// surface. A nil result is zero counts (e.g. the thesis has no embedding/subjects).
func (s *Service) GetContradictions(ctx context.Context, thesisClaimID string) (db.ClaimStanceCounts, error) {
	return s.store.ContradictionSurface(ctx, thesisClaimID, claimResolveMinCosine)
}

// resolveClaim maps a claim to a canonical claim id + the source's stance on it. Order: exact
// text → hybrid candidate retrieval (pgvector + graph, RRF-fused) → bounded LLM adjudication →
// create. We adjudicate the new claim against at most claimAdjudicateMax fused candidates (NOT
// every existing claim), with early-exit: the first "same" reuses the canonical (assert), the
// first "negation" reuses it as a deny (the contradiction mechanism); "related" candidates
// accumulate as proposed argument edges on a newly-created claim.
func (s *Service) resolveClaim(ctx context.Context, scope, owner, trust, text, polarity string, subjectIDs []string, sourceID string) (string, string, error) {
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

	if s.adjudicator != nil {
		cands := s.candidateClaims(ctx, scope, owner, subjectIDs, vec)
		var related []relatedEdge
		for i, cand := range cands {
			if i >= claimAdjudicateMax {
				break // cost ceiling — never adjudicate the whole candidate set
			}
			rel, jerr := s.adjudicator.JudgeClaims(ctx, llm.ClaimAdjudicationInput{A: text, B: cand.text})
			if jerr != nil {
				slog.Warn("claims: adjudicator failed, skipping candidate", "component", "claims", "err", jerr)
				continue
			}
			switch rel.Relation {
			case "same":
				return cand.id, "assert", nil
			case "negation":
				return cand.id, "deny", nil // the contradiction mechanism
			case "related":
				edge := rel.EdgeType
				if edge == "" || edge == "none" {
					edge = "qualifies"
				}
				related = append(related, relatedEdge{to: cand.id, typ: edge, conf: rel.Confidence})
			}
		}
		if len(related) > 0 {
			id, err := s.createClaim(ctx, scope, owner, trust, text, polarity, sourceID, vec)
			if err != nil {
				return "", "", err
			}
			for _, r := range related {
				if _, err := s.store.AddClaimEdge(ctx, db.ClaimEdgeParams{
					FromClaimID: id, ToClaimID: r.to, Type: r.typ, Confidence: r.conf, Method: "llm",
				}); err != nil {
					return "", "", err
				}
			}
			return id, "assert", nil
		}
	}

	id, err := s.createClaim(ctx, scope, owner, trust, text, polarity, sourceID, vec)
	return id, "assert", err
}

type fusedCand struct {
	id   string
	text string
}

type relatedEdge struct {
	to   string
	typ  string
	conf float64
}

// candidateClaims gathers resolution candidates from both channels — pgvector (same subject,
// textually near) and the graph (subject + 1-hop neighbours) — and fuses them with RRF. Either
// channel may be absent (no embedding / no graph configured); the fusion uses what's available.
func (s *Service) candidateClaims(ctx context.Context, scope, owner string, subjectIDs []string, vec []float32) []fusedCand {
	var vecList []db.ScoredClaim
	if len(vec) > 0 {
		if cs, err := s.store.FindClaimCandidates(ctx, scope, owner, subjectIDs, vec, claimResolveMinCosine, claimCandidateLimit); err != nil {
			slog.Warn("claims: vector candidate retrieval failed", "component", "claims", "err", err)
		} else {
			vecList = cs
		}
	}
	var graphList []graph.ClaimCandidate
	if s.graphCands != nil && len(subjectIDs) > 0 {
		if gs, err := s.graphCands.ClaimCandidates(ctx, subjectIDs, claimGraphLimit); err != nil {
			slog.Warn("claims: graph candidate retrieval failed", "component", "claims", "err", err)
		} else {
			graphList = gs
		}
	}
	return fuseRRF(vecList, graphList)
}

// fuseRRF merges two ranked candidate lists with Reciprocal Rank Fusion: an item's score is the
// sum over channels of 1/(rrfK + rank). No cross-channel score normalization (cosine and graph
// distance aren't comparable); a claim appearing in BOTH channels is a strong signal and RRF
// rewards it. Returns candidates best-first, deduped by claim id.
func fuseRRF(vecList []db.ScoredClaim, graphList []graph.ClaimCandidate) []fusedCand {
	score := make(map[string]float64)
	text := make(map[string]string)
	for rank, c := range vecList {
		score[c.ID] += 1.0 / float64(rrfK+rank+1)
		if text[c.ID] == "" {
			text[c.ID] = c.CanonicalText
		}
	}
	for rank, c := range graphList {
		score[c.ID] += 1.0 / float64(rrfK+rank+1)
		if text[c.ID] == "" {
			text[c.ID] = c.Text
		}
	}
	ids := make([]string, 0, len(score))
	for id := range score {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if score[ids[i]] != score[ids[j]] {
			return score[ids[i]] > score[ids[j]]
		}
		return ids[i] < ids[j] // deterministic tiebreak
	})
	out := make([]fusedCand, 0, len(ids))
	for _, id := range ids {
		out = append(out, fusedCand{id: id, text: text[id]})
	}
	return out
}

func (s *Service) createClaim(ctx context.Context, scope, owner, trust, text, polarity, sourceID string, vec []float32) (string, error) {
	id, err := s.store.CreateClaim(ctx, db.CreateClaimParams{
		Scope: scope, OwnerUserID: owner, CanonicalText: text, Polarity: polarity, TrustTier: trust, FirstSourceID: sourceID,
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
