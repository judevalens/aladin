package workers

import (
	"context"
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/ratelimit"
)

type fakeEnricher struct {
	result *llm.EnrichResult
	err    error
}

func (f *fakeEnricher) Enrich(ctx context.Context, content, recordType string) (*llm.EnrichResult, error) {
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

func TestGlobalFirstPassWorkerEnrichesRecord(t *testing.T) {
	t.Parallel()

	w := NewGlobalFirstPassWorker(&fakeEnricher{
		result: &llm.EnrichResult{
			Summary:   "source summary",
			Entities:  []string{"Hacker News"},
			Topics:    []string{"agents"},
			KeyClaims: []string{"claim"},
		},
	}, ratelimit.New(1000))

	raw, _ := json.Marshal(pipeline.GlobalRecordPayload{
		RecordID:         "record-1",
		ProviderStreamID: "stream-1",
		Type:             "post",
		ContentExcerpt:   "raw post excerpt",
		ContextExcerpt:   "hydrated context excerpt",
	})

	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if result.Type != pipeline.ResultGlobalFirstPassDone {
		t.Fatalf("result.Type = %q, want %q", result.Type, pipeline.ResultGlobalFirstPassDone)
	}
	var payload pipeline.GlobalRecordPayload
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		t.Fatalf("unmarshal result payload: %v", err)
	}
	if payload.Summary != "source summary" || len(payload.Entities) != 1 {
		t.Fatalf("payload enrichment = %#v, want summary and entities", payload)
	}
}

// embedFakeRepo is a minimal db.RecordRepository for the embed worker test.
type embedFakeRepo struct {
	rec      *db.Record
	savedVec []float32
	savedRev int64
}

func (r *embedFakeRepo) Get(_ context.Context, _ string) (*db.Record, error) { return r.rec, nil }
func (r *embedFakeRepo) SaveEmbedding(_ context.Context, _ string, rev int64, vec []float32) (bool, error) {
	r.savedVec = vec
	r.savedRev = rev
	return true, nil
}
func (r *embedFakeRepo) UpsertCanonical(_ context.Context, _ *db.Record) (*db.RecordUpsertResult, error) {
	return nil, nil
}
func (r *embedFakeRepo) SaveEnrichment(_ context.Context, _ *db.RecordEnrichment) (bool, error) {
	return true, nil
}
func (r *embedFakeRepo) SaveComplete(_ context.Context, _ *db.CompletedRecord) error { return nil }
func (r *embedFakeRepo) ListStuck(_ context.Context, _, _ int) ([]*db.Record, error) {
	return nil, nil
}
func (r *embedFakeRepo) MarkFailed(_ context.Context, _, _ string) error         { return nil }
func (r *embedFakeRepo) ResetForRetry(_ context.Context, _ string) (bool, error) { return false, nil }

func TestEmbedWorkerWritesEmbeddingAndCompletes(t *testing.T) {
	t.Parallel()

	repo := &embedFakeRepo{rec: &db.Record{
		ID:             "record-4",
		SourceRevision: 1,
		Title:          "label",
		ContentExcerpt: "content",
		Enrichment:     db.RecordEnrichment{Summary: "summary"},
	}}
	w := NewEmbedWorker(repo, &fakeEmbedder{vector: []float32{1, 2, 3}}, ratelimit.New(1000))

	raw, _ := json.Marshal(pipeline.ResolveEntitiesPayload{RecordID: "record-4", SourceRevision: 1})
	result := w.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if result.Type != pipeline.ResultEmbedDone {
		t.Fatalf("result.Type = %q, want %q", result.Type, pipeline.ResultEmbedDone)
	}
	if len(repo.savedVec) != 3 || repo.savedRev != 1 {
		t.Fatalf("embedding not saved: vec=%v rev=%d", repo.savedVec, repo.savedRev)
	}
}
