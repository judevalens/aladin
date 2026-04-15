package workers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/search"
)

type fakeEnricher struct {
	result *llm.EnrichResult
	err    error
}

func (f *fakeEnricher) Enrich(ctx context.Context, content, artifactType string) (*llm.EnrichResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type fakeEmbedder struct {
	vector []float32
	err    error
}

func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vector, nil
}

type fakeSearcher struct {
	results map[string][]search.SearchResult
	errs    map[string]error
}

func (f *fakeSearcher) Search(ctx context.Context, query string, maxResults int) ([]search.SearchResult, error) {
	if err := f.errs[query]; err != nil {
		return nil, err
	}
	return f.results[query], nil
}

func TestFirstPassWorkerReturnsEmbedReady(t *testing.T) {
	t.Parallel()

	w := NewFirstPassWorker(&fakeEnricher{
		result: &llm.EnrichResult{
			Summary: "summary",
			Entities: []string{"entity"},
			Topics: []string{"topic"},
			KeyClaims: []string{"claim"},
		},
	}, ratelimit.New(1000))

	raw, _ := json.Marshal(pipeline.ArtifactPayload{
		ArtifactID: "artifact-1",
		KgID:       "kg-1",
		Type:       "post",
		Content:    "hello",
	})

	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if result.Type != pipeline.ResultFirstPassEmbedReady {
		t.Fatalf("result.Type = %q, want %q", result.Type, pipeline.ResultFirstPassEmbedReady)
	}
}

func TestFirstPassWorkerReturnsSearchNeeded(t *testing.T) {
	t.Parallel()

	w := NewFirstPassWorker(&fakeEnricher{
		result: &llm.EnrichResult{
			Summary:               "summary",
			LowConfidenceEntities: []string{"entity"},
		},
	}, ratelimit.New(1000))

	raw, _ := json.Marshal(pipeline.ArtifactPayload{
		ArtifactID: "artifact-2",
		KgID:       "kg-2",
		Type:       "post",
		Content:    "hello",
	})

	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if result.Type != pipeline.ResultFirstPassSearchNeeded {
		t.Fatalf("result.Type = %q, want %q", result.Type, pipeline.ResultFirstPassSearchNeeded)
	}
}

func TestSearchWorkerClassifiesHTTPErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want any
	}{
		{name: "rate_limit", err: &search.ErrHTTP{StatusCode: 429}, want: pipeline.ErrRateLimit{}},
		{name: "permanent", err: &search.ErrHTTP{StatusCode: 401}, want: pipeline.ErrPermanent{}},
		{name: "transient", err: &search.ErrHTTP{StatusCode: 500}, want: pipeline.ErrTransient{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := NewSearchWorker(&fakeSearcher{errs: map[string]error{"entity": tt.err}})
			raw, _ := json.Marshal(pipeline.ArtifactPayload{
				ArtifactID:           "artifact",
				KgID:                 "kg",
				LowConfidenceEntities: []string{"entity"},
			})

			result := w.Run(context.Background(), raw)
			if result.Err == nil {
				t.Fatal("Run returned nil error, want classified error")
			}
			switch tt.want.(type) {
			case pipeline.ErrRateLimit:
				var got pipeline.ErrRateLimit
				if !errors.As(result.Err, &got) {
					t.Fatalf("error = %T, want ErrRateLimit", result.Err)
				}
			case pipeline.ErrPermanent:
				var got pipeline.ErrPermanent
				if !errors.As(result.Err, &got) {
					t.Fatalf("error = %T, want ErrPermanent", result.Err)
				}
			case pipeline.ErrTransient:
				var got pipeline.ErrTransient
				if !errors.As(result.Err, &got) {
					t.Fatalf("error = %T, want ErrTransient", result.Err)
				}
			}
		})
	}
}

func TestSearchWorkerTransientErrorCarriesNoPayload(t *testing.T) {
	t.Parallel()

	w := NewSearchWorker(&fakeSearcher{
		results: map[string][]search.SearchResult{
			"done": {{Title: "done"}},
		},
		errs: map[string]error{
			"next": errors.New("network"),
		},
	})

	raw, _ := json.Marshal(pipeline.ArtifactPayload{
		ArtifactID:            "artifact-3",
		KgID:                  "kg-3",
		LowConfidenceEntities: []string{"done", "next"},
	})

	result := w.Run(context.Background(), raw)
	var got pipeline.ErrTransient
	if !errors.As(result.Err, &got) {
		t.Fatalf("error = %T, want ErrTransient", result.Err)
	}
	if result.Payload != nil {
		t.Fatalf("expected no payload on transient error, got %d bytes", len(result.Payload))
	}
}

func TestEmbedWorkerReturnsEmbedding(t *testing.T) {
	t.Parallel()

	w := NewEmbedWorker(&fakeEmbedder{vector: []float32{1, 2, 3}}, ratelimit.New(1000))
	raw, _ := json.Marshal(pipeline.ArtifactPayload{
		ArtifactID: "artifact-4",
		KgID:       "kg-4",
		Label:      "label",
		Summary:    "summary",
		Content:    "content",
	})

	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if result.Type != pipeline.ResultEmbedDone {
		t.Fatalf("result.Type = %q, want %q", result.Type, pipeline.ResultEmbedDone)
	}
}

type graphPromoterFake struct {
	called bool
	err    error
}

func (f *graphPromoterFake) Promote(ctx context.Context, artifact *db.EmbeddedArtifact) error {
	f.called = true
	return f.err
}

func TestGraphWorkerPromotesAndCompletes(t *testing.T) {
	t.Parallel()

	promoter := &graphPromoterFake{}
	w := NewGraphWorker(promoter)
	raw, _ := json.Marshal(pipeline.ArtifactPayload{
		ArtifactID: "artifact-5",
		KgID:       "kg-5",
		Type:       "post",
		Label:      "label",
		SourceURL:  "https://example.com",
		Summary:    "summary",
	})

	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if !promoter.called {
		t.Fatal("Promote was not called")
	}
	if result.Type != pipeline.ResultGraphDone {
		t.Fatalf("result.Type = %q, want %q", result.Type, pipeline.ResultGraphDone)
	}
}
