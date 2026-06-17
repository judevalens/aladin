package entities

import (
	"context"
	"strconv"
	"testing"

	"aladin/backend_v2/internal/db"
)

// fakeStore is an in-memory db.EntityRepository for resolver unit tests.
type fakeStore struct {
	byKey    map[string]string // "kind|norm" -> entity id
	created  int
	aliases  int
	mentions []db.MentionParams
	seq      int
}

func newFakeStore() *fakeStore { return &fakeStore{byKey: map[string]string{}} }

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
