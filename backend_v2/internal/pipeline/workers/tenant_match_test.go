package workers

import (
	"context"
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

type tenantMatchRecordRepo struct {
	record *db.Record
}

func (r tenantMatchRecordRepo) SaveComplete(ctx context.Context, a *db.CompletedRecord) error {
	return nil
}

func (r tenantMatchRecordRepo) UpsertCanonical(ctx context.Context, record *db.Record) (*db.RecordUpsertResult, error) {
	return nil, nil
}

func (r tenantMatchRecordRepo) Get(ctx context.Context, id string) (*db.Record, error) {
	return r.record, nil
}

func (r tenantMatchRecordRepo) SaveEnrichment(ctx context.Context, enrichment *db.RecordEnrichment) (bool, error) {
	return true, nil
}

func (r tenantMatchRecordRepo) SaveEmbedding(ctx context.Context, recordID string, sourceRevision int64, vec []float32) (bool, error) {
	return true, nil
}

func (r tenantMatchRecordRepo) ListStuck(ctx context.Context, olderThanSecs, limit int) ([]*db.Record, error) {
	return nil, nil
}

func (r tenantMatchRecordRepo) MarkFailed(ctx context.Context, recordID, reason string) error {
	return nil
}

func (r tenantMatchRecordRepo) ResetForRetry(ctx context.Context, recordID string) (bool, error) {
	return false, nil
}

type tenantMatchStreamRepo struct {
	stream *db.ProviderStream
}

func (r tenantMatchStreamRepo) GetByID(ctx context.Context, id string) (*db.ProviderStream, error) {
	return r.stream, nil
}

func (r tenantMatchStreamRepo) Ensure(ctx context.Context, stream *db.ProviderStream) (*db.ProviderStream, error) {
	return nil, nil
}

func (r tenantMatchStreamRepo) ClaimBatch(ctx context.Context, limit int) ([]*db.ProviderStream, error) {
	return nil, nil
}

func (r tenantMatchStreamRepo) MarkSyncStarted(ctx context.Context, id string) error { return nil }
func (r tenantMatchStreamRepo) Heartbeat(ctx context.Context, id string) error        { return nil }
func (r tenantMatchStreamRepo) MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	return nil
}
func (r tenantMatchStreamRepo) MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	return nil
}
func (r tenantMatchStreamRepo) MarkSyncFailed(ctx context.Context, id string) error { return nil }
func (r tenantMatchStreamRepo) Release(ctx context.Context, id string) error        { return nil }

type tenantMatchSubRepo struct {
	subs []*db.SourceSubscription
}

func (r tenantMatchSubRepo) ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*db.SourceSubscription, error) {
	return r.subs, nil
}

func (r tenantMatchSubRepo) Ensure(ctx context.Context, sub *db.SourceSubscription) (*db.SourceSubscription, error) {
	return nil, nil
}

type tenantMatchRepo struct {
	saved []*db.TenantItemMatch
}

func (r *tenantMatchRepo) Save(ctx context.Context, match *db.TenantItemMatch) error {
	r.saved = append(r.saved, match)
	return nil
}

func TestTenantMatchWorkerSavesRelevantMatchAndEmitsInsightTrigger(t *testing.T) {
	t.Parallel()

	matches := &tenantMatchRepo{}
	worker := NewTenantMatchWorker(
		tenantMatchRecordRepo{record: &db.Record{
			ID:               "record-1",
			ProviderStreamID: "stream-1",
			Provider:         "bluesky",
			Type:             "post",
			Title:            "AI agents are useful",
			SourceRevision:   3,
			Enrichment: db.RecordEnrichment{
				Topics: []string{"agents"},
			},
		}},
		tenantMatchStreamRepo{stream: &db.ProviderStream{ID: "stream-1", Config: map[string]any{}}},
		tenantMatchSubRepo{subs: []*db.SourceSubscription{{
			ID:               "sub-1",
			UserID:           "user-1",
			KgID:             "kg-1",
			ProviderStreamID: "stream-1",
			Policy: map[string]any{
				"keywords":           []any{"agents"},
				"insight_generators": []any{"topic_trend"},
			},
		}}},
		matches,
	)

	raw, _ := json.Marshal(pipeline.TenantMatchPayload{
		RecordID:       "record-1",
		CorrelationID:  "corr-1",
		SourceRevision: 3,
	})
	result := worker.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if len(matches.saved) != 1 {
		t.Fatalf("saved matches = %d, want 1", len(matches.saved))
	}
	if matches.saved[0].RelevanceStatus != string(RelevanceRelevant) {
		t.Fatalf("match = %+v", matches.saved[0])
	}
	var payload pipeline.TenantMatchPayload
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(payload.InsightTriggers) != 1 {
		t.Fatalf("insight triggers = %d, want 1", len(payload.InsightTriggers))
	}
	if got := payload.InsightTriggers[0]; got.KgID != "kg-1" || got.RecordID != "record-1" || len(got.GeneratorKeys) != 1 {
		t.Fatalf("insight trigger = %+v", got)
	}
}

func TestTenantMatchWorkerNoDecisionDoesNotPersistMatch(t *testing.T) {
	t.Parallel()

	matches := &tenantMatchRepo{}
	worker := NewTenantMatchWorker(
		tenantMatchRecordRepo{record: &db.Record{
			ID:               "record-1",
			ProviderStreamID: "stream-1",
			Provider:         "bluesky",
			Type:             "post",
			Title:            "gardening",
			SourceRevision:   3,
		}},
		tenantMatchStreamRepo{stream: &db.ProviderStream{ID: "stream-1", Config: map[string]any{}}},
		tenantMatchSubRepo{subs: []*db.SourceSubscription{{
			ID:               "sub-1",
			UserID:           "user-1",
			KgID:             "kg-1",
			ProviderStreamID: "stream-1",
			Policy:           map[string]any{"keywords": []any{"agents"}},
		}}},
		matches,
	)

	raw, _ := json.Marshal(pipeline.TenantMatchPayload{RecordID: "record-1"})
	result := worker.Run(context.Background(), raw)
	if result.Err != nil {
		t.Fatalf("Run returned error: %v", result.Err)
	}
	if len(matches.saved) != 0 {
		t.Fatalf("saved matches = %d, want 0", len(matches.saved))
	}
}
