package claims

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

// fakeClaimStore is an in-memory db.ClaimRepository for unit tests.
type fakeClaimStore struct {
	byText   map[string]string // canonical_text -> claim id
	created  int
	subjects map[string][]string // claim id -> entity ids
	mentions []db.ClaimMentionParams
	seq      int
}

func newFakeClaimStore() *fakeClaimStore {
	return &fakeClaimStore{byText: map[string]string{}, subjects: map[string][]string{}}
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

func itoa(n int) string { return string(rune('0' + n)) } // small n only

// fakeExtractor returns canned claims.
type fakeExtractor struct {
	claims []llm.ExtractedClaim
	err    error
}

func (f fakeExtractor) ExtractClaims(_ context.Context, _ llm.ClaimExtractionInput) ([]llm.ExtractedClaim, error) {
	return f.claims, f.err
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
