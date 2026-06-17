package entities

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

// fakeStore is an in-memory db.EntityRepository for resolver unit tests.
type fakeStore struct {
	byKey         map[string]string // "kind|norm" -> entity id
	created       int
	aliases       int
	mentions      []db.MentionParams
	seq           int
	candidates    []db.ScoredCandidate    // returned by FindSharedCandidates
	proposed      []db.ProposeMergeParams // recorded by ProposeMerge
	rejectedPairs map[string]bool         // "from|into" negative evidence
	distinct      [][2]string             // recorded by RecordDistinct
	tenantByKey    map[string]string      // "owner|kind|norm" -> tenant entity id
	lastTenantBind string                 // SharedEntityID of the last CreateTenantEntity
	embeddings     map[string][]float32   // entity id -> stored embedding
}

func newFakeStore() *fakeStore {
	return &fakeStore{byKey: map[string]string{}, rejectedPairs: map[string]bool{}, tenantByKey: map[string]string{}, embeddings: map[string][]float32{}}
}

func storeKey(kind, norm string) string { return kind + "|" + norm }

func (f *fakeStore) FindSharedByKey(_ context.Context, kind, norm string) ([]db.EntityCandidate, error) {
	if id, ok := f.byKey[storeKey(kind, norm)]; ok {
		return []db.EntityCandidate{{ID: id, Kind: kind, NormalizedKey: norm}}, nil
	}
	return nil, nil
}

func (f *fakeStore) CreateSharedEntity(_ context.Context, p db.CreateEntityParams) (string, error) {
	f.seq++
	id := "e" + strconv.Itoa(f.seq)
	f.byKey[storeKey(p.Kind, p.NormalizedKey)] = id
	f.created++
	return id, nil
}

func (f *fakeStore) AddAlias(_ context.Context, _ db.AliasParams) error { f.aliases++; return nil }

func (f *fakeStore) AddMention(_ context.Context, p db.MentionParams) error {
	f.mentions = append(f.mentions, p)
	return nil
}

func (f *fakeStore) FindSharedCandidates(_ context.Context, _, _ string, minSim float64, _ int) ([]db.ScoredCandidate, error) {
	var out []db.ScoredCandidate
	for _, c := range f.candidates {
		if c.Similarity >= minSim {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) ProposeMerge(_ context.Context, p db.ProposeMergeParams) (bool, error) {
	if f.rejectedPairs[p.FromEntityID+"|"+p.IntoEntityID] || f.rejectedPairs[p.IntoEntityID+"|"+p.FromEntityID] {
		return false, nil
	}
	f.proposed = append(f.proposed, p)
	return true, nil
}

func (f *fakeStore) RecordDistinct(_ context.Context, from, into, _, _ string) error {
	f.distinct = append(f.distinct, [2]string{from, into})
	f.rejectedPairs[from+"|"+into] = true
	return nil
}

func (f *fakeStore) ListProposedMerges(_ context.Context, _ int) ([]db.ProposedMerge, error) {
	return nil, nil
}
func (f *fakeStore) RejectMerge(_ context.Context, _ string) error { return nil }
func (f *fakeStore) AcceptMerge(_ context.Context, _ string) error { return nil }
func (f *fakeStore) RevertMerge(_ context.Context, _ string) error { return nil }

func (f *fakeStore) FindTenantByKey(_ context.Context, owner, kind, norm string) ([]db.EntityCandidate, error) {
	if id, ok := f.tenantByKey[owner+"|"+kind+"|"+norm]; ok {
		return []db.EntityCandidate{{ID: id, Kind: kind, NormalizedKey: norm}}, nil
	}
	return nil, nil
}

func (f *fakeStore) CreateTenantEntity(_ context.Context, p db.CreateTenantEntityParams) (string, error) {
	f.seq++
	id := "t" + strconv.Itoa(f.seq)
	f.tenantByKey[p.OwnerUserID+"|"+p.Kind+"|"+p.NormalizedKey] = id
	f.lastTenantBind = p.SharedEntityID
	return id, nil
}

func (f *fakeStore) ResolveCanonicalRoot(_ context.Context, entityID string) (string, error) {
	return entityID, nil
}

func (f *fakeStore) SetEntityEmbedding(_ context.Context, entityID string, vec []float32) error {
	f.embeddings[entityID] = vec
	return nil
}
func (f *fakeStore) GetEntityEmbedding(_ context.Context, entityID string) ([]float32, bool, error) {
	v, ok := f.embeddings[entityID]
	return v, ok, nil
}
func (f *fakeStore) FindSharedCandidatesByVector(_ context.Context, _, _ string, _ []float32, _ float64, _ int) ([]db.ScoredCandidate, error) {
	return nil, nil
}

// fakeEmbedder returns deterministic vectors keyed by the embed text.
type fakeEmbedder struct{ vecs map[string][]float32 }

func (f fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if v, ok := f.vecs[text]; ok {
		return v, nil
	}
	return []float32{0.5, 0.5}, nil
}

func TestResolver_SenseSplitOnDivergentContext(t *testing.T) {
	emb := fakeEmbedder{vecs: map[string][]float32{
		"Mercury — chemistry":         {1, 0},
		"Mercury — automobile brand":  {0, 1},
	}}
	s := newFakeStore()
	r := NewResolver(s).WithEmbedder(emb)

	id1, err := r.Resolve(context.Background(), Mention{Surface: "Mercury", ContextHint: "chemistry", RecordID: "r"})
	if err != nil {
		t.Fatalf("resolve 1: %v", err)
	}
	id2, err := r.Resolve(context.Background(), Mention{Surface: "Mercury", ContextHint: "automobile brand", RecordID: "r"})
	if err != nil {
		t.Fatalf("resolve 2: %v", err)
	}
	if id1 == id2 {
		t.Fatal("divergent context for the same name+kind must split into a new sense")
	}
	if s.created != 2 {
		t.Fatalf("expected 2 entities (a split), got %d", s.created)
	}
}

func TestResolver_SameSenseResolvesToExisting(t *testing.T) {
	emb := fakeEmbedder{vecs: map[string][]float32{
		"Mercury — the element":      {1, 0},
		"Mercury — chemical element": {0.99, 0.01},
	}}
	s := newFakeStore()
	r := NewResolver(s).WithEmbedder(emb)

	id1, err := r.Resolve(context.Background(), Mention{Surface: "Mercury", ContextHint: "the element", RecordID: "r"})
	if err != nil {
		t.Fatalf("resolve 1: %v", err)
	}
	id2, err := r.Resolve(context.Background(), Mention{Surface: "Mercury", ContextHint: "chemical element", RecordID: "r"})
	if err != nil {
		t.Fatalf("resolve 2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("similar context must resolve to the same sense: %s vs %s", id1, id2)
	}
	if s.created != 1 {
		t.Fatalf("expected 1 entity (no split), got %d", s.created)
	}
}

func TestResolver_TenantBindsToSharedWhenPresent(t *testing.T) {
	s := newFakeStore()
	s.byKey[storeKey("unknown", "acme")] = "shared-acme" // a shared canonical exists
	r := NewResolver(s)

	id, err := r.ResolveTenant(context.Background(), "owner-1", Mention{Surface: "Acme", RecordID: "r"})
	if err != nil {
		t.Fatalf("resolve tenant: %v", err)
	}
	if id == "shared-acme" {
		t.Fatal("tenant resolve must create a tenant overlay entity, not return the shared id")
	}
	if s.lastTenantBind != "shared-acme" {
		t.Fatalf("expected the tenant entity bound to the shared canonical, got bind=%q", s.lastTenantBind)
	}
}

func TestResolver_TenantCreatesPrivateWhenNoShared(t *testing.T) {
	s := newFakeStore()
	r := NewResolver(s)
	if _, err := r.ResolveTenant(context.Background(), "owner-1", Mention{Surface: "MyPrivateNote", RecordID: "r"}); err != nil {
		t.Fatalf("resolve tenant: %v", err)
	}
	if s.lastTenantBind != "" {
		t.Fatalf("expected an unbound (private) tenant entity, got bind=%q", s.lastTenantBind)
	}
}

// fakeAdjudicator is a deterministic llm.EntityAdjudicator for unit tests.
type fakeAdjudicator struct {
	verdict string
	conf    float64
	err     error
}

func (a fakeAdjudicator) JudgeSameEntity(_ context.Context, _ llm.EntityAdjudicationInput) (*llm.EntityVerdict, error) {
	if a.err != nil {
		return nil, a.err
	}
	return &llm.EntityVerdict{Verdict: a.verdict, Confidence: a.conf, Reason: "test"}, nil
}

func TestResolver_AdjudicatorDifferentSuppressesProposal(t *testing.T) {
	s := newFakeStore()
	s.candidates = []db.ScoredCandidate{{ID: "existing", CanonicalName: "Apex Legal", NormalizedKey: "apex legal", Similarity: 0.6}}
	r := NewResolver(s).WithAdjudicator(fakeAdjudicator{verdict: "different", conf: 0.9})
	if _, err := r.Resolve(context.Background(), Mention{Surface: "Apex Financial", RecordID: "r"}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(s.proposed) != 0 {
		t.Fatalf("a 'different' verdict must suppress the proposal, got %d", len(s.proposed))
	}
	if len(s.distinct) != 1 {
		t.Fatalf("a 'different' verdict must record negative evidence, got %d", len(s.distinct))
	}
}

func TestResolver_AdjudicatorSameProposesWithLLMMethod(t *testing.T) {
	s := newFakeStore()
	s.candidates = []db.ScoredCandidate{{ID: "ibm", CanonicalName: "International Business Machines", NormalizedKey: "international business machines", Similarity: 0.5}}
	r := NewResolver(s).WithAdjudicator(fakeAdjudicator{verdict: "same", conf: 0.95})
	if _, err := r.Resolve(context.Background(), Mention{Surface: "IBM", RecordID: "r"}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(s.proposed) != 1 || s.proposed[0].Method != "llm" {
		t.Fatalf("a 'same' verdict must propose with method=llm, got %+v", s.proposed)
	}
}

func TestResolver_AdjudicatorErrorFallsBackToTrigram(t *testing.T) {
	s := newFakeStore()
	s.candidates = []db.ScoredCandidate{{ID: "x", CanonicalName: "Foobar", NormalizedKey: "foobar", Similarity: 0.7}}
	r := NewResolver(s).WithAdjudicator(fakeAdjudicator{err: errors.New("no api key")})
	if _, err := r.Resolve(context.Background(), Mention{Surface: "Foobaz", RecordID: "r"}); err != nil {
		t.Fatalf("resolve must not fail when the adjudicator errors: %v", err)
	}
	if len(s.proposed) != 1 || s.proposed[0].Method != "trigram" {
		t.Fatalf("adjudicator error must degrade to a trigram proposal, got %+v", s.proposed)
	}
}

func TestResolver_FuzzyNearMatchProposesMergeNeverAutoMerges(t *testing.T) {
	s := newFakeStore()
	// An existing "Anthropic" is a strong trigram candidate for a new "Anthropics".
	s.candidates = []db.ScoredCandidate{
		{ID: "existing", Kind: "unknown", CanonicalName: "Anthropic", NormalizedKey: "anthropic", Similarity: 0.8},
	}
	r := NewResolver(s)

	id, err := r.Resolve(context.Background(), Mention{Surface: "Anthropics", RecordID: "r"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id == "existing" {
		t.Fatal("R1 must NOT auto-merge a fuzzy match — it should create a new entity and propose")
	}
	if s.created != 1 {
		t.Fatalf("expected a new entity created, got %d", s.created)
	}
	if len(s.proposed) != 1 || s.proposed[0].IntoEntityID != "existing" {
		t.Fatalf("expected 1 proposed merge into the candidate, got %+v", s.proposed)
	}
}

func TestResolver_NoFuzzyCandidateNoProposal(t *testing.T) {
	s := newFakeStore() // no candidates seeded
	r := NewResolver(s)
	if _, err := r.Resolve(context.Background(), Mention{Surface: "Mistral", RecordID: "r"}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(s.proposed) != 0 {
		t.Fatalf("expected no proposed merges, got %d", len(s.proposed))
	}
}

func TestResolver_CollapsesVariantsToOneEntity(t *testing.T) {
	s := newFakeStore()
	r := NewResolver(s)
	ids := map[string]bool{}
	for _, surface := range []string{"OpenAI", "OpenAI Inc", "OpenAI, Inc.", "openai"} {
		id, err := r.Resolve(context.Background(), Mention{Surface: surface, RecordID: "rec1"})
		if err != nil {
			t.Fatalf("resolve %q: %v", surface, err)
		}
		ids[id] = true
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 canonical entity, got %d: %v", len(ids), ids)
	}
	if s.created != 1 {
		t.Fatalf("expected 1 entity created, got %d", s.created)
	}
	if len(s.mentions) != 4 {
		t.Fatalf("expected 4 mentions recorded, got %d", len(s.mentions))
	}
}

func TestResolver_DistinctNamesDistinctEntities(t *testing.T) {
	s := newFakeStore()
	r := NewResolver(s)
	a, _ := r.Resolve(context.Background(), Mention{Surface: "OpenAI", RecordID: "r"})
	b, _ := r.Resolve(context.Background(), Mention{Surface: "Anthropic", RecordID: "r"})
	if a == b {
		t.Fatalf("expected distinct entities for distinct names, both = %q", a)
	}
	if s.created != 2 {
		t.Fatalf("expected 2 entities created, got %d", s.created)
	}
}

func TestResolver_SkipsEmptySurface(t *testing.T) {
	s := newFakeStore()
	r := NewResolver(s)
	id, err := r.Resolve(context.Background(), Mention{Surface: "   ", RecordID: "r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" || s.created != 0 || len(s.mentions) != 0 {
		t.Fatalf("expected empty surface to be skipped, got id=%q created=%d mentions=%d", id, s.created, len(s.mentions))
	}
}
