package workers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/websearch"
)

type fakeResolver struct{ calls []entities.Mention }

func (f *fakeResolver) Resolve(_ context.Context, m entities.Mention) (string, error) {
	f.calls = append(f.calls, m)
	return "ent-" + m.Surface, nil
}

type fakeSearcher struct {
	results map[string][]websearch.SearchResult
	err     error
}

func (f *fakeSearcher) Search(_ context.Context, q string, _ int) ([]websearch.SearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results[q], nil
}

func lowConfRecord(low ...string) *embedFakeRepo {
	return &embedFakeRepo{rec: &db.Record{
		ID:             "r1",
		SourceRevision: 1,
		Enrichment:     db.RecordEnrichment{Summary: "a summary", LowConfidenceEntities: low},
	}}
}

func TestResolveLowConfidence_SearchesAndResolves(t *testing.T) {
	t.Parallel()

	repo := lowConfRecord("Acme Niche")
	resolver := &fakeResolver{}
	searcher := &fakeSearcher{results: map[string][]websearch.SearchResult{
		"Acme Niche": {{Title: "Acme Niche Inc", Content: "Acme Niche is a robotics startup."}},
	}}
	w := NewResolveLowConfidenceWorker(repo, resolver, searcher, nil)

	raw, _ := json.Marshal(pipeline.ResolveEntitiesPayload{RecordID: "r1", SourceRevision: 1})
	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run error: %v", result.Err)
	}
	if result.Type != pipeline.ResultResolveLowConfidenceDone {
		t.Fatalf("result.Type = %q", result.Type)
	}
	if len(resolver.calls) != 1 || resolver.calls[0].Surface != "Acme Niche" {
		t.Fatalf("resolver calls = %+v", resolver.calls)
	}
	if !strings.Contains(resolver.calls[0].ContextHint, "robotics startup") {
		t.Fatalf("context hint should carry the search snippet, got %q", resolver.calls[0].ContextHint)
	}
}

func TestResolveLowConfidence_NilSearcherNoOp(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{}
	w := NewResolveLowConfidenceWorker(lowConfRecord("X"), resolver, nil, nil)
	result := w.Run(context.Background(), mustPayload())
	if result.Err != nil {
		t.Fatalf("Run error: %v", result.Err)
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("nil searcher must no-op, got %d resolve calls", len(resolver.calls))
	}
}

func TestResolveLowConfidence_SearchErrorIsBestEffort(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{}
	searcher := &fakeSearcher{err: errors.New("tavily down")}
	w := NewResolveLowConfidenceWorker(lowConfRecord("Acme Niche"), resolver, searcher, nil)

	result := w.Run(context.Background(), mustPayload())
	if result.Err != nil {
		t.Fatalf("a search miss must not fail the record: %v", result.Err)
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("should still resolve without web context, got %d calls", len(resolver.calls))
	}
	if resolver.calls[0].ContextHint != "a summary" {
		t.Fatalf("hint should fall back to the summary, got %q", resolver.calls[0].ContextHint)
	}
}

func mustPayload() []byte {
	raw, _ := json.Marshal(pipeline.ResolveEntitiesPayload{RecordID: "r1", SourceRevision: 1})
	return raw
}
