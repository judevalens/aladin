package entities

import (
	"context"
	"errors"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/websearch"
)

// ── fakes ────────────────────────────────────────────────────────────────────────────

type fakeEnt struct{ name, kind string }

type fakeJudgeStore struct {
	entities     map[string]fakeEnt
	placeholders []db.PlaceholderEntity
	candidates   map[string][]db.ScoredCandidate
	judgeable    []db.JudgeableMerge
	proposals    []db.ProposeMergeParams
	decided      map[string]string // mergeID → outcome
	decidedBy    map[string]string
	roots        map[string]string // entityID → root (unset = self)
	promoted     map[string]bool
	aliases      map[string][]string
	nextMergeID  int
}

func newFakeJudgeStore() *fakeJudgeStore {
	return &fakeJudgeStore{
		entities:   map[string]fakeEnt{},
		candidates: map[string][]db.ScoredCandidate{},
		decided:    map[string]string{},
		decidedBy:  map[string]string{},
		roots:      map[string]string{},
		promoted:   map[string]bool{},
		aliases:    map[string][]string{},
	}
}

func (f *fakeJudgeStore) ListPlaceholders(_ context.Context, _ int) ([]db.PlaceholderEntity, error) {
	return f.placeholders, nil
}

func (f *fakeJudgeStore) FindMergeCandidatesAnyKind(_ context.Context, entityID, _ string, _ float64, _ int) ([]db.ScoredCandidate, error) {
	return f.candidates[entityID], nil
}

// ProposeMerge mirrors the real one loosely: appends, and surfaces the pair as a
// judgeable merge so a single Sweep exercises the whole placeholder→judged loop.
func (f *fakeJudgeStore) ProposeMerge(_ context.Context, p db.ProposeMergeParams) (bool, error) {
	for _, ex := range f.proposals {
		if (ex.FromEntityID == p.FromEntityID && ex.IntoEntityID == p.IntoEntityID) ||
			(ex.FromEntityID == p.IntoEntityID && ex.IntoEntityID == p.FromEntityID) {
			return false, nil
		}
	}
	f.proposals = append(f.proposals, p)
	f.nextMergeID++
	from, into := f.entities[p.FromEntityID], f.entities[p.IntoEntityID]
	f.judgeable = append(f.judgeable, db.JudgeableMerge{
		ID:     itoa(f.nextMergeID),
		FromID: p.FromEntityID, FromName: from.name, FromKind: from.kind,
		IntoID: p.IntoEntityID, IntoName: into.name, IntoKind: into.kind,
		Confidence: p.Confidence, Method: p.Method,
	})
	return true, nil
}

func itoa(n int) string { return string(rune('a' + n)) }

func (f *fakeJudgeStore) ListJudgeableMerges(_ context.Context, _ int) ([]db.JudgeableMerge, error) {
	var out []db.JudgeableMerge
	for _, m := range f.judgeable {
		if _, done := f.decided[m.ID]; !done {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeJudgeStore) DecideMerge(_ context.Context, mergeID, outcome, decidedBy string, _ map[string]any) error {
	f.decided[mergeID] = outcome
	f.decidedBy[mergeID] = decidedBy
	if outcome == "applied" {
		for _, m := range f.judgeable {
			if m.ID == mergeID {
				f.roots[m.FromID] = m.IntoID
			}
		}
	}
	return nil
}

func (f *fakeJudgeStore) HasProposedMergeFor(_ context.Context, entityID string) (bool, error) {
	for _, m := range f.judgeable {
		if f.decided[m.ID] != "" && f.decided[m.ID] != "unsure" {
			continue
		}
		if m.FromID == entityID || m.IntoID == entityID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeJudgeStore) PromoteEntity(_ context.Context, entityID string) error {
	f.promoted[entityID] = true
	return nil
}

func (f *fakeJudgeStore) ListEntityAliases(_ context.Context, entityID string) ([]string, error) {
	return f.aliases[entityID], nil
}

func (f *fakeJudgeStore) ResolveCanonicalRoot(_ context.Context, entityID string) (string, error) {
	if r, ok := f.roots[entityID]; ok {
		return r, nil
	}
	return entityID, nil
}

type fakeJudge struct {
	verdicts []llm.EntityVerdict
	calls    []llm.EntityAdjudicationInput
}

func (f *fakeJudge) JudgeSameEntity(_ context.Context, in llm.EntityAdjudicationInput) (*llm.EntityVerdict, error) {
	f.calls = append(f.calls, in)
	v := f.verdicts[0]
	if len(f.verdicts) > 1 {
		f.verdicts = f.verdicts[1:]
	}
	return &v, nil
}

type fakeJudgeSearcher struct {
	results []websearch.SearchResult
	err     error
	queries []string
}

func (f *fakeJudgeSearcher) Search(_ context.Context, q string, _ int) ([]websearch.SearchResult, error) {
	f.queries = append(f.queries, q)
	return f.results, f.err
}

// ── tests ────────────────────────────────────────────────────────────────────────────

// A placeholder with no candidates anywhere is a real, distinct entity → promoted.
func TestJudgeSweep_PlaceholderNoCandidatesPromotes(t *testing.T) {
	s := newFakeJudgeStore()
	s.placeholders = []db.PlaceholderEntity{{ID: "p1", Kind: "other", CanonicalName: "Ghostco", NormalizedKey: "ghostco"}}

	n, err := NewJudgeSweeper(s, nil).Sweep(context.Background(), 10)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !s.promoted["p1"] || n == 0 {
		t.Fatalf("expected placeholder promoted, promoted=%v acted=%d", s.promoted, n)
	}
}

// A placeholder whose surface exactly matches an existing entity's alias (sim 1.0,
// kinds compatible: other vs org) merges deterministically — no LLM involved.
func TestJudgeSweep_PlaceholderExactAliasAutoMerges(t *testing.T) {
	s := newFakeJudgeStore()
	s.entities["p1"] = fakeEnt{"nvda", "other"}
	s.entities["e1"] = fakeEnt{"Nvidia", "org"}
	s.placeholders = []db.PlaceholderEntity{{ID: "p1", Kind: "other", CanonicalName: "nvda", NormalizedKey: "nvda"}}
	s.candidates["p1"] = []db.ScoredCandidate{{ID: "e1", Kind: "org", CanonicalName: "Nvidia", Similarity: 1.0}}

	if _, err := NewJudgeSweeper(s, nil).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(s.decided) != 1 {
		t.Fatalf("expected one decided merge, got %v", s.decided)
	}
	for id, outcome := range s.decided {
		if outcome != "applied" || s.decidedBy[id] != "auto" {
			t.Fatalf("expected auto-applied, got outcome=%s by=%s", outcome, s.decidedBy[id])
		}
	}
	if s.promoted["p1"] {
		t.Fatal("merged placeholder must not be promoted")
	}
}

// Identical names across two TYPED kinds are a potential sense split — never
// deterministic. The judge decides (here: same → applied by llm).
func TestJudgeSweep_CrossTypedKindGoesToJudge(t *testing.T) {
	s := newFakeJudgeStore()
	s.judgeable = []db.JudgeableMerge{{
		ID: "m1", FromID: "a", FromName: "Apple", FromKind: "org",
		IntoID: "b", IntoName: "Apple", IntoKind: "concept", Confidence: 1.0,
	}}
	judge := &fakeJudge{verdicts: []llm.EntityVerdict{{Verdict: "same", Confidence: 0.9}}}

	if _, err := NewJudgeSweeper(s, judge).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(judge.calls) != 1 {
		t.Fatalf("expected the judge consulted exactly once, got %d", len(judge.calls))
	}
	if s.decided["m1"] != "applied" || s.decidedBy["m1"] != "llm" {
		t.Fatalf("expected llm-applied, got %s by %s", s.decided["m1"], s.decidedBy["m1"])
	}
}

// Judge "different" → rejected (negative evidence).
func TestJudgeSweep_JudgeDifferentRejects(t *testing.T) {
	s := newFakeJudgeStore()
	s.judgeable = []db.JudgeableMerge{{
		ID: "m1", FromID: "a", FromName: "NVDA", FromKind: "other",
		IntoID: "b", IntoName: "AMD", IntoKind: "org", Confidence: 0.5,
	}}
	judge := &fakeJudge{verdicts: []llm.EntityVerdict{{Verdict: "different", Confidence: 0.95}}}

	if _, err := NewJudgeSweeper(s, judge).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if s.decided["m1"] != "rejected" {
		t.Fatalf("expected rejected, got %s", s.decided["m1"])
	}
}

// Unsure with no searcher: marked unsure (stays proposed, never re-judged), and a
// placeholder held by the pending pair is NOT promoted.
func TestJudgeSweep_UnsureWithoutSearcherHolds(t *testing.T) {
	s := newFakeJudgeStore()
	s.entities["p1"] = fakeEnt{"HDFC", "other"}
	s.entities["e1"] = fakeEnt{"HDFC Bank", "org"}
	s.placeholders = []db.PlaceholderEntity{{ID: "p1", Kind: "other", CanonicalName: "HDFC", NormalizedKey: "hdfc"}}
	s.candidates["p1"] = []db.ScoredCandidate{{ID: "e1", Kind: "org", CanonicalName: "HDFC Bank", Similarity: 0.6}}
	judge := &fakeJudge{verdicts: []llm.EntityVerdict{{Verdict: "uncertain", Confidence: 0.4}}}

	if _, err := NewJudgeSweeper(s, judge).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	var outcome string
	for _, o := range s.decided {
		outcome = o
	}
	if outcome != "unsure" {
		t.Fatalf("expected unsure, got %q", outcome)
	}
	if s.promoted["p1"] {
		t.Fatal("placeholder with a pending pair must not be promoted")
	}
}

// Unsure + searcher: web context fetched for both names, ONE re-judge with the
// snippets as ContextHint; second verdict decides.
func TestJudgeSweep_UnsureEscalatesToWebAndResolves(t *testing.T) {
	s := newFakeJudgeStore()
	s.judgeable = []db.JudgeableMerge{{
		ID: "m1", FromID: "a", FromName: "HDFC", FromKind: "other",
		IntoID: "b", IntoName: "HDFC Bank", IntoKind: "org", Confidence: 0.6,
	}}
	judge := &fakeJudge{verdicts: []llm.EntityVerdict{
		{Verdict: "uncertain", Confidence: 0.4},
		{Verdict: "same", Confidence: 0.9},
	}}
	searcher := &fakeJudgeSearcher{results: []websearch.SearchResult{{Content: "HDFC Bank, India's largest private bank, commonly called HDFC"}}}

	if _, err := NewJudgeSweeper(s, judge).WithSearcher(searcher).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(searcher.queries) != 2 {
		t.Fatalf("expected both names searched, got %v", searcher.queries)
	}
	if len(judge.calls) != 2 {
		t.Fatalf("expected exactly one re-judge, got %d calls", len(judge.calls))
	}
	if judge.calls[1].ContextHint == "" {
		t.Fatal("re-judge must carry the web snippets as ContextHint")
	}
	if s.decided["m1"] != "applied" {
		t.Fatalf("expected applied after escalation, got %s", s.decided["m1"])
	}
}

// Searcher failure degrades gracefully: the first (unsure) verdict stands.
func TestJudgeSweep_SearcherErrorDegradesToUnsure(t *testing.T) {
	s := newFakeJudgeStore()
	s.judgeable = []db.JudgeableMerge{{
		ID: "m1", FromID: "a", FromName: "X", FromKind: "other",
		IntoID: "b", IntoName: "Y", IntoKind: "org", Confidence: 0.6,
	}}
	judge := &fakeJudge{verdicts: []llm.EntityVerdict{{Verdict: "uncertain", Confidence: 0.4}}}
	searcher := &fakeJudgeSearcher{err: errors.New("tavily down")}

	if _, err := NewJudgeSweeper(s, judge).WithSearcher(searcher).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if s.decided["m1"] != "unsure" {
		t.Fatalf("expected unsure on search failure, got %s", s.decided["m1"])
	}
	if len(judge.calls) != 1 {
		t.Fatalf("expected no re-judge without context, got %d calls", len(judge.calls))
	}
}

// No judge configured: only the deterministic tier acts; sub-threshold rows are left
// untouched (proposed, un-judged) rather than guessed at.
func TestJudgeSweep_NilJudgeDeterministicOnly(t *testing.T) {
	s := newFakeJudgeStore()
	s.judgeable = []db.JudgeableMerge{
		{ID: "m1", FromID: "a", FromName: "Nvidia Corp", FromKind: "org",
			IntoID: "b", IntoName: "Nvidia", IntoKind: "org", Confidence: 0.95},
		{ID: "m2", FromID: "c", FromName: "HDFC", FromKind: "other",
			IntoID: "d", IntoName: "HDFC Bank", IntoKind: "org", Confidence: 0.6},
	}

	if _, err := NewJudgeSweeper(s, nil).Sweep(context.Background(), 10); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if s.decided["m1"] != "applied" || s.decidedBy["m1"] != "auto" {
		t.Fatalf("expected m1 auto-applied, got %s by %s", s.decided["m1"], s.decidedBy["m1"])
	}
	if _, touched := s.decided["m2"]; touched {
		t.Fatalf("expected m2 untouched without a judge, got %s", s.decided["m2"])
	}
}
