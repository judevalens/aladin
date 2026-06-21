package claims

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/llm"
)

// fakeGraphSource is an in-memory GraphClaimSource — the graph retrieval channel.
type fakeGraphSource struct {
	cands       []graph.ClaimCandidate
	gotSubjects []string
}

func (f *fakeGraphSource) ClaimCandidates(_ context.Context, subjectIDs []string, _ int) ([]graph.ClaimCandidate, error) {
	f.gotSubjects = subjectIDs
	return f.cands, nil
}

// countingAdj records how many times the adjudicator was called (the LLM-cost signal).
type countingAdj struct {
	rel   string
	edge  string
	calls int
}

func (c *countingAdj) JudgeClaims(_ context.Context, _ llm.ClaimAdjudicationInput) (*llm.ClaimRelation, error) {
	c.calls++
	return &llm.ClaimRelation{Relation: c.rel, EdgeType: c.edge, Confidence: 0.5}, nil
}

// A graph-channel candidate (with no vector candidate at all) can drive resolution — proving the
// second channel is actually consulted and fused.
func TestService_GraphChannelCandidateResolves(t *testing.T) {
	store := newFakeClaimStore() // no vector candidates
	gs := &fakeGraphSource{cands: []graph.ClaimCandidate{{ID: "existing", Text: "OpenAI's approach will win"}}}
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is going to win with its approach", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).
		WithEmbedder(fakeClaimEmbedder{}).
		WithAdjudicator(&countingAdj{rel: "same"}).
		WithGraphCandidates(gs)

	if _, err := svc.ExtractFromRecord(context.Background(), recordInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(gs.gotSubjects) != 1 || gs.gotSubjects[0] != "e-openai" {
		t.Fatalf("graph channel must be queried with the resolved subject ids, got %v", gs.gotSubjects)
	}
	if store.created != 0 {
		t.Fatalf("a graph candidate judged 'same' must reuse it, not create (created=%d)", store.created)
	}
	if len(store.mentions) != 1 || store.mentions[0].ClaimID != "existing" {
		t.Fatalf("expected the assert mention on the graph candidate, got %+v", store.mentions)
	}
}

// The cap: we never adjudicate the new claim against more than claimAdjudicateMax candidates,
// even when retrieval returns many. This is the LLM-cost ceiling.
func TestService_AdjudicationIsCapped(t *testing.T) {
	store := newFakeClaimStore()
	var many []graph.ClaimCandidate
	for _, id := range []string{"g1", "g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9", "g10"} {
		many = append(many, graph.ClaimCandidate{ID: id, Text: "a distinct neighbour claim " + id})
	}
	gs := &fakeGraphSource{cands: many}
	adj := &countingAdj{rel: "none"} // never resolves → would walk the whole list if uncapped
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is burning cash", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).
		WithEmbedder(fakeClaimEmbedder{}).
		WithAdjudicator(adj).
		WithGraphCandidates(gs)

	if _, err := svc.ExtractFromRecord(context.Background(), recordInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if adj.calls != claimAdjudicateMax {
		t.Fatalf("adjudications = %d, want capped at %d", adj.calls, claimAdjudicateMax)
	}
	if store.created != 1 {
		t.Fatalf("no candidate matched → exactly one new claim, created=%d", store.created)
	}
}

// Early-exit: the first "same" stops adjudication immediately (no wasted LLM calls).
func TestService_AdjudicationEarlyExits(t *testing.T) {
	store := newFakeClaimStore()
	gs := &fakeGraphSource{cands: []graph.ClaimCandidate{
		{ID: "g1", Text: "first"}, {ID: "g2", Text: "second"}, {ID: "g3", Text: "third"},
	}}
	adj := &countingAdj{rel: "same"} // first candidate already resolves
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI claim", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).
		WithEmbedder(fakeClaimEmbedder{}).
		WithAdjudicator(adj).
		WithGraphCandidates(gs)

	if _, err := svc.ExtractFromRecord(context.Background(), recordInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if adj.calls != 1 {
		t.Fatalf("a 'same' on the top candidate must stop after 1 adjudication, got %d", adj.calls)
	}
}

// RRF rewards a claim that appears in BOTH channels by ranking it first.
func TestFuseRRF_OverlapRanksFirst(t *testing.T) {
	vec := []db.ScoredClaim{{ID: "x", CanonicalText: "x"}, {ID: "y", CanonicalText: "y"}}
	g := []graph.ClaimCandidate{{ID: "y", Text: "y"}, {ID: "z", Text: "z"}}

	out := fuseRRF(vec, g)
	if len(out) != 3 {
		t.Fatalf("fused candidates = %d, want 3 (deduped union)", len(out))
	}
	if out[0].id != "y" {
		t.Fatalf("the claim in both channels must rank first, got %q", out[0].id)
	}
}

// Without a graph source, retrieval is pgvector-only — exactly the prior behaviour.
func TestService_NilGraphSourceIsVectorOnly(t *testing.T) {
	store := newFakeClaimStore()
	store.candidates = []db.ScoredClaim{{ID: "existing", CanonicalText: "OpenAI's approach will win", Similarity: 0.95}}
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is going to win", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).WithEmbedder(fakeClaimEmbedder{}).WithAdjudicator(&countingAdj{rel: "same"})

	if _, err := svc.ExtractFromRecord(context.Background(), recordInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if store.created != 0 || len(store.mentions) != 1 || store.mentions[0].ClaimID != "existing" {
		t.Fatalf("vector-only path must still resolve to the candidate, got created=%d mentions=%+v", store.created, store.mentions)
	}
}
