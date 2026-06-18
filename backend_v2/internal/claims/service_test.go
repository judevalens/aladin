package claims

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

// fakeClaimStore is an in-memory db.ClaimRepository for unit tests.
type fakeClaimStore struct {
	byText     map[string]string // canonical_text -> claim id
	created    int
	subjects   map[string][]string // claim id -> entity ids
	mentions   []db.ClaimMentionParams
	seq        int
	embeddings map[string][]float32
	candidates []db.ScoredClaim // returned by FindClaimCandidates
	edges      []db.ClaimEdgeParams
}

func newFakeClaimStore() *fakeClaimStore {
	return &fakeClaimStore{byText: map[string]string{}, subjects: map[string][]string{}, embeddings: map[string][]float32{}}
}

func (f *fakeClaimStore) FindClaimByText(_ context.Context, _, _, text string) (string, bool, error) {
	id, ok := f.byText[text]
	return id, ok, nil
}
func (f *fakeClaimStore) CreateClaim(_ context.Context, p db.CreateClaimParams) (string, error) {
	f.seq++
	id := "c" + itoa(f.seq)
	f.byText[p.CanonicalText] = id
	f.created++
	return id, nil
}
func (f *fakeClaimStore) AddClaimSubject(_ context.Context, claimID, entityID string) error {
	f.subjects[claimID] = append(f.subjects[claimID], entityID)
	return nil
}
func (f *fakeClaimStore) AddClaimMention(_ context.Context, p db.ClaimMentionParams) error {
	f.mentions = append(f.mentions, p)
	return nil
}

func (f *fakeClaimStore) SetClaimEmbedding(_ context.Context, claimID string, vec []float32) error {
	f.embeddings[claimID] = vec
	return nil
}
func (f *fakeClaimStore) FindClaimCandidates(_ context.Context, _, _ string, _ []string, _ []float32, _ float64, _ int) ([]db.ScoredClaim, error) {
	return f.candidates, nil
}
func (f *fakeClaimStore) AddClaimEdge(_ context.Context, p db.ClaimEdgeParams) (bool, error) {
	f.edges = append(f.edges, p)
	return true, nil
}
func (f *fakeClaimStore) ContradictionSurface(_ context.Context, _ string, _ float64) (db.ClaimStanceCounts, error) {
	return db.ClaimStanceCounts{}, nil
}

func itoa(n int) string { return string(rune('0' + n)) } // small n only

// fakeExtractor returns canned claims.
type fakeExtractor struct {
	claims []llm.ExtractedClaim
	err    error
}

func (f fakeExtractor) ExtractClaims(_ context.Context, _ llm.ClaimExtractionInput) ([]llm.ExtractedClaim, error) {
	return f.claims, f.err
}

type fakeClaimEmbedder struct{}

func (fakeClaimEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{1, 0}, nil
}

type fakeClaimAdj struct {
	rel  string
	edge string
}

func (f fakeClaimAdj) JudgeClaims(_ context.Context, _ llm.ClaimAdjudicationInput) (*llm.ClaimRelation, error) {
	return &llm.ClaimRelation{Relation: f.rel, EdgeType: f.edge, Confidence: 0.9}, nil
}

func negationInput() RecordInput {
	in := recordInput()
	in.KeyClaims = []string{"OpenAI's approach will not win"}
	return in
}

func TestService_NegationResolvesToDenyMention(t *testing.T) {
	store := newFakeClaimStore()
	store.candidates = []db.ScoredClaim{{ID: "existing", CanonicalText: "OpenAI's approach will win", Polarity: "assert", Similarity: 0.9}}
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI's approach will not win", Polarity: "deny", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).WithEmbedder(fakeClaimEmbedder{}).WithAdjudicator(fakeClaimAdj{rel: "negation"})

	if _, err := svc.ExtractFromRecord(context.Background(), negationInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if store.created != 0 {
		t.Fatalf("a negation must reuse the canonical claim, not create one (created=%d)", store.created)
	}
	if len(store.mentions) != 1 || store.mentions[0].Stance != "deny" || store.mentions[0].ClaimID != "existing" {
		t.Fatalf("expected one deny mention on the canonical, got %+v", store.mentions)
	}
}

func TestService_SameResolvesToAssert(t *testing.T) {
	store := newFakeClaimStore()
	store.candidates = []db.ScoredClaim{{ID: "existing", CanonicalText: "OpenAI's approach will win", Polarity: "assert", Similarity: 0.95}}
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is going to win with its approach", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).WithEmbedder(fakeClaimEmbedder{}).WithAdjudicator(fakeClaimAdj{rel: "same"})

	if _, err := svc.ExtractFromRecord(context.Background(), recordInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if store.created != 0 {
		t.Fatalf("a paraphrase must reuse the canonical, not create (created=%d)", store.created)
	}
	if len(store.mentions) != 1 || store.mentions[0].Stance != "assert" {
		t.Fatalf("expected one assert mention, got %+v", store.mentions)
	}
}

func TestService_RelatedCreatesEdge(t *testing.T) {
	store := newFakeClaimStore()
	store.candidates = []db.ScoredClaim{{ID: "existing", CanonicalText: "OpenAI's approach will win", Polarity: "assert", Similarity: 0.82}}
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is burning too much cash", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	svc := NewService(store, ext).WithEmbedder(fakeClaimEmbedder{}).WithAdjudicator(fakeClaimAdj{rel: "related", edge: "contradicts"})

	if _, err := svc.ExtractFromRecord(context.Background(), recordInput()); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if store.created != 1 {
		t.Fatalf("a related claim must be a new claim, created=%d", store.created)
	}
	if len(store.edges) != 1 || store.edges[0].Type != "contradicts" || store.edges[0].ToClaimID != "existing" {
		t.Fatalf("expected one contradicts edge to the candidate, got %+v", store.edges)
	}
}

func recordInput() RecordInput {
	return RecordInput{
		RecordID:  "rec1",
		Summary:   "a post about OpenAI",
		KeyClaims: []string{"OpenAI's approach will win"},
		Entities:  []db.EntityRef{{ID: "e-openai", Name: "OpenAI"}},
	}
}

func TestService_StoresContestableGroundedClaim(t *testing.T) {
	store := newFakeClaimStore()
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI's approach will win", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	n, err := NewService(store, ext).ExtractFromRecord(context.Background(), recordInput())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if n != 1 || store.created != 1 {
		t.Fatalf("expected 1 claim stored, got n=%d created=%d", n, store.created)
	}
	if len(store.mentions) != 1 || store.mentions[0].Stance != "assert" {
		t.Fatalf("expected 1 asserting mention, got %+v", store.mentions)
	}
	if got := store.subjects["c1"]; len(got) != 1 || got[0] != "e-openai" {
		t.Fatalf("expected the claim grounded on e-openai, got %v", got)
	}
}

func TestService_SkipsNonContestable(t *testing.T) {
	store := newFakeClaimStore()
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI is a company", Polarity: "assert", Contestable: false, SubjectNames: []string{"OpenAI"}},
	}}
	n, _ := NewService(store, ext).ExtractFromRecord(context.Background(), recordInput())
	if n != 0 || store.created != 0 {
		t.Fatalf("a non-contestable fact must be skipped, got n=%d created=%d", n, store.created)
	}
}

func TestService_SkipsUngrounded(t *testing.T) {
	store := newFakeClaimStore()
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "Crypto will replace banks", Polarity: "assert", Contestable: true, SubjectNames: []string{"Bitcoin"}}, // not in the record's entities
	}}
	n, _ := NewService(store, ext).ExtractFromRecord(context.Background(), recordInput())
	if n != 0 || store.created != 0 {
		t.Fatalf("a claim about an un-resolved entity must be skipped, got n=%d created=%d", n, store.created)
	}
}

func TestService_DedupsByExactText(t *testing.T) {
	store := newFakeClaimStore()
	ext := fakeExtractor{claims: []llm.ExtractedClaim{
		{Text: "OpenAI's approach will win", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
		{Text: "OpenAI's approach will win", Polarity: "assert", Contestable: true, SubjectNames: []string{"OpenAI"}},
	}}
	n, _ := NewService(store, ext).ExtractFromRecord(context.Background(), recordInput())
	if store.created != 1 {
		t.Fatalf("identical text must dedup to one claim, created=%d", store.created)
	}
	_ = n
}

func TestService_NilExtractorIsNoOp(t *testing.T) {
	store := newFakeClaimStore()
	n, err := NewService(store, nil).ExtractFromRecord(context.Background(), recordInput())
	if err != nil || n != 0 || store.created != 0 {
		t.Fatalf("nil extractor must be a no-op, got n=%d err=%v", n, err)
	}
}
