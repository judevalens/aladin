package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
)

type fakeEnqueuer struct {
	stageCalls []stageCall
	stageErr   error
	// delayedStageCalls tracks calls made outside the interface for test assertions.
	delayedStageCalls []delayedStageCall
}

type stageCall struct {
	taskType string
	recordID string
	payload  []byte
}

type delayedStageCall struct {
	taskType string
	recordID string
	payload  []byte
	delay    time.Duration
}

func (f *fakeEnqueuer) EnqueueStage(ctx context.Context, taskType, recordID string, payload []byte) error {
	f.stageCalls = append(f.stageCalls, stageCall{taskType: taskType, recordID: recordID, payload: payload})
	return f.stageErr
}

type fakeRecordRepo struct {
	saved []*db.CompletedRecord
	err   error
}

func (f *fakeRecordRepo) SaveComplete(ctx context.Context, a *db.CompletedRecord) error {
	f.saved = append(f.saved, a)
	return f.err
}

func (f *fakeRecordRepo) ExistsExternal(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	return nil, errors.New("not implemented")
}

func TestFullPipelineHandlerRoutesFirstPassSearchNeeded(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeRecordRepo{}, make(chan string, 1))

	err := h.OnDone(context.Background(), Result{
		Type:     ResultFirstPassSearchNeeded,
		TaskType: TaskFirstPass,
		RecordID: "record-1",
		Payload:  []byte(`{"record_id":"record-1"}`),
	})
	if err != nil {
		t.Fatalf("OnDone returned error: %v", err)
	}
	if len(enq.stageCalls) != 1 {
		t.Fatalf("EnqueueStage calls = %d, want 1", len(enq.stageCalls))
	}
	if got := enq.stageCalls[0]; got.taskType != TaskSearch || got.recordID != "record-1" {
		t.Fatalf("EnqueueStage call = %+v, want task=%q record=%q", got, TaskSearch, "record-1")
	}
}

func TestFullPipelineHandlerRateLimitBubblesError(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeRecordRepo{}, make(chan string, 1))

	rlErr := ErrRateLimit{RetryAfter: 45 * time.Second}
	err := h.OnDone(context.Background(), Result{
		TaskType: TaskSearch,
		RecordID: "record-2",
		Payload:  []byte(`{"record_id":"record-2"}`),
		Err:      rlErr,
	})
	if err == nil {
		t.Fatal("OnDone returned nil, want rate limit error")
	}
	var got ErrRateLimit
	if !errors.As(err, &got) || got.RetryAfter != 45*time.Second {
		t.Fatalf("OnDone error = %v, want ErrRateLimit{RetryAfter: 45s}", err)
	}
	if len(enq.stageCalls) != 0 || len(enq.delayedStageCalls) != 0 {
		t.Fatalf("unexpected enqueue calls on rate limit: %+v %+v", enq.stageCalls, enq.delayedStageCalls)
	}
}

func TestFullPipelineHandlerPermanentErrorDrops(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeRecordRepo{}, make(chan string, 1))

	err := h.OnDone(context.Background(), Result{
		TaskType: TaskEmbed,
		RecordID: "record-3",
		Err:      ErrPermanent{Cause: errors.New("bad input")},
	})
	if err != nil {
		t.Fatalf("OnDone returned error: %v", err)
	}
	if len(enq.stageCalls) != 0 || len(enq.delayedStageCalls) != 0 {
		t.Fatalf("unexpected enqueue calls: %+v %+v", enq.stageCalls, enq.delayedStageCalls)
	}
}

func TestFullPipelineHandlerTransientErrorBubbles(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeRecordRepo{}, make(chan string, 1))

	want := ErrTransient{Cause: errors.New("temporary")}
	err := h.OnDone(context.Background(), Result{
		TaskType: TaskGraph,
		RecordID: "record-4",
		Err:      want,
	})
	if err == nil {
		t.Fatal("OnDone returned nil error, want transient error")
	}
	if !errors.Is(err, want.Cause) {
		t.Fatalf("OnDone error = %v, want wrapped %v", err, want.Cause)
	}
}

func TestFullPipelineHandlerPersistsCompletedRecord(t *testing.T) {
	t.Parallel()

	repo := &fakeRecordRepo{}
	insights := make(chan string, 1)
	h := NewFullPipelineHandler(&fakeEnqueuer{}, repo, insights)

	payload, err := json.Marshal(RecordPayload{
		RecordID:       "record-5",
		KgID:           "kg-5",
		SourceID:       "source-5",
		ExternalID:     "ext-5",
		SourceRevision: 42,
		Type:           "post",
		Label:          "label",
		Content:        "content",
		SourceURL:      "https://example.com",
		Metadata:       map[string]any{"score": float64(7)},
		Summary:        "summary",
		Entities:       []string{"entity"},
		Topics:         []string{"topic"},
		KeyClaims:      []string{"claim"},
		Embedding:      []float32{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	err = h.OnDone(context.Background(), Result{
		Type:     ResultGraphDone,
		TaskType: TaskGraph,
		RecordID: "record-5",
		KgID:     "kg-5",
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("OnDone returned error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("SaveComplete calls = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "record-5" || repo.saved[0].ExternalID != "ext-5" {
		t.Fatalf("saved record = %+v", repo.saved[0])
	}
	if repo.saved[0].SourceRevision != 42 {
		t.Fatalf("saved source revision = %d, want 42", repo.saved[0].SourceRevision)
	}
	select {
	case kgID := <-insights:
		if kgID != "kg-5" {
			t.Fatalf("insight kgID = %q, want kg-5", kgID)
		}
	default:
		t.Fatal("expected insight signal")
	}
}
