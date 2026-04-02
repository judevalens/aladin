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
	stageCalls        []stageCall
	delayedStageCalls []delayedStageCall
	stageErr          error
	delayedErr        error
}

type stageCall struct {
	taskType   string
	artifactID string
	payload    []byte
}

type delayedStageCall struct {
	taskType   string
	artifactID string
	payload    []byte
	delay      time.Duration
}

func (f *fakeEnqueuer) EnqueueStage(ctx context.Context, taskType, artifactID string, payload []byte) error {
	f.stageCalls = append(f.stageCalls, stageCall{taskType: taskType, artifactID: artifactID, payload: payload})
	return f.stageErr
}

func (f *fakeEnqueuer) EnqueueStageDelayed(ctx context.Context, taskType, artifactID string, payload []byte, delay time.Duration) error {
	f.delayedStageCalls = append(f.delayedStageCalls, delayedStageCall{taskType: taskType, artifactID: artifactID, payload: payload, delay: delay})
	return f.delayedErr
}

type fakeArtifactRepo struct {
	saved []*db.CompletedArtifact
	err   error
}

func (f *fakeArtifactRepo) SaveComplete(ctx context.Context, a *db.CompletedArtifact) error {
	f.saved = append(f.saved, a)
	return f.err
}

func (f *fakeArtifactRepo) ExistsExternal(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	return nil, errors.New("not implemented")
}

func TestFullPipelineHandlerRoutesFirstPassSearchNeeded(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeArtifactRepo{}, make(chan string, 1))

	err := h.OnDone(context.Background(), Result{
		Type:       ResultFirstPassSearchNeeded,
		TaskType:   TaskFirstPass,
		ArtifactID: "artifact-1",
		Payload:    []byte(`{"artifact_id":"artifact-1"}`),
	})
	if err != nil {
		t.Fatalf("OnDone returned error: %v", err)
	}
	if len(enq.stageCalls) != 1 {
		t.Fatalf("EnqueueStage calls = %d, want 1", len(enq.stageCalls))
	}
	if got := enq.stageCalls[0]; got.taskType != TaskSearch || got.artifactID != "artifact-1" {
		t.Fatalf("EnqueueStage call = %+v, want task=%q artifact=%q", got, TaskSearch, "artifact-1")
	}
}

func TestFullPipelineHandlerRateLimitReenqueue(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeArtifactRepo{}, make(chan string, 1))

	err := h.OnDone(context.Background(), Result{
		TaskType:   TaskSearch,
		ArtifactID: "artifact-2",
		Payload:    []byte(`{"artifact_id":"artifact-2"}`),
		Err:        ErrRateLimit{RetryAfter: 45 * time.Second},
	})
	if err != nil {
		t.Fatalf("OnDone returned error: %v", err)
	}
	if len(enq.delayedStageCalls) != 1 {
		t.Fatalf("EnqueueStageDelayed calls = %d, want 1", len(enq.delayedStageCalls))
	}
	call := enq.delayedStageCalls[0]
	if call.taskType != TaskSearch || call.artifactID != "artifact-2" || call.delay != 45*time.Second {
		t.Fatalf("EnqueueStageDelayed call = %+v", call)
	}
}

func TestFullPipelineHandlerPermanentErrorDrops(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	h := NewFullPipelineHandler(enq, &fakeArtifactRepo{}, make(chan string, 1))

	err := h.OnDone(context.Background(), Result{
		TaskType:   TaskEmbed,
		ArtifactID: "artifact-3",
		Err:        ErrPermanent{Cause: errors.New("bad input")},
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
	h := NewFullPipelineHandler(enq, &fakeArtifactRepo{}, make(chan string, 1))

	want := ErrTransient{Cause: errors.New("temporary")}
	err := h.OnDone(context.Background(), Result{
		TaskType:   TaskGraph,
		ArtifactID: "artifact-4",
		Err:        want,
	})
	if err == nil {
		t.Fatal("OnDone returned nil error, want transient error")
	}
	if !errors.Is(err, want.Cause) {
		t.Fatalf("OnDone error = %v, want wrapped %v", err, want.Cause)
	}
}

func TestFullPipelineHandlerPersistsCompletedArtifact(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepo{}
	insights := make(chan string, 1)
	h := NewFullPipelineHandler(&fakeEnqueuer{}, repo, insights)

	payload, err := json.Marshal(ArtifactPayload{
		ArtifactID: "artifact-5",
		KgID:       "kg-5",
		SourceID:   "source-5",
		ExternalID: "ext-5",
		Type:       "post",
		Label:      "label",
		Content:    "content",
		SourceURL:  "https://example.com",
		Metadata:   map[string]any{"score": float64(7)},
		Summary:    "summary",
		Entities:   []string{"entity"},
		Topics:     []string{"topic"},
		KeyClaims:  []string{"claim"},
		Embedding:  []float32{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	err = h.OnDone(context.Background(), Result{
		Type:       ResultGraphDone,
		TaskType:   TaskGraph,
		ArtifactID: "artifact-5",
		KgID:       "kg-5",
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("OnDone returned error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("SaveComplete calls = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].ID != "artifact-5" || repo.saved[0].ExternalID != "ext-5" {
		t.Fatalf("saved artifact = %+v", repo.saved[0])
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
