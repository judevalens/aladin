package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
)

type fakeSourceRepo struct {
	claimed        []*db.Source
	claimErr       error
	markFailedIDs  []string
	markStartedIDs []string
	markSyncedIDs  []string
	releasedIDs    []string
	markFailedErr  error
	markStartedErr error
	markSyncedErr  error
}

func (f *fakeSourceRepo) GetByID(ctx context.Context, id string) (*db.Source, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSourceRepo) ClaimBatch(ctx context.Context, limit int) ([]*db.Source, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return f.claimed, nil
}

func (f *fakeSourceRepo) MarkSyncStarted(ctx context.Context, id string) error {
	f.markStartedIDs = append(f.markStartedIDs, id)
	return f.markStartedErr
}

func (f *fakeSourceRepo) MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	return nil
}

func (f *fakeSourceRepo) MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	f.markSyncedIDs = append(f.markSyncedIDs, id)
	return f.markSyncedErr
}

func (f *fakeSourceRepo) MarkSyncFailed(ctx context.Context, id string) error {
	f.markFailedIDs = append(f.markFailedIDs, id)
	return f.markFailedErr
}

func (f *fakeSourceRepo) Release(ctx context.Context, id string) error {
	f.releasedIDs = append(f.releasedIDs, id)
	return nil
}

type fakeCycleRepo struct {
	cyclesBySource   map[string][]*db.SyncCycle
	created          []*db.SyncCycle
	runningIDs       []string
	updatedIDs       []string
	updatedHeads     []map[string]any
	completedIDs     []string
	completedHeads   []map[string]any
	completedReasons []string
}

func (f *fakeCycleRepo) ListActiveBySource(ctx context.Context, sourceID string) ([]*db.SyncCycle, error) {
	if f.cyclesBySource == nil {
		return nil, nil
	}
	return f.cyclesBySource[sourceID], nil
}

func (f *fakeCycleRepo) Create(ctx context.Context, cycle *db.SyncCycle) error {
	f.created = append(f.created, cycle)
	return nil
}

func (f *fakeCycleRepo) MarkRunning(ctx context.Context, id string) error {
	f.runningIDs = append(f.runningIDs, id)
	return nil
}

func (f *fakeCycleRepo) UpdateProgress(ctx context.Context, id string, cursor map[string]any, headBoundary map[string]any) error {
	f.updatedIDs = append(f.updatedIDs, id)
	f.updatedHeads = append(f.updatedHeads, headBoundary)
	return nil
}

func (f *fakeCycleRepo) MarkActive(ctx context.Context, id string) error {
	return nil
}

func (f *fakeCycleRepo) Complete(ctx context.Context, id string, headBoundary map[string]any, completionReason string) error {
	f.completedIDs = append(f.completedIDs, id)
	f.completedHeads = append(f.completedHeads, headBoundary)
	f.completedReasons = append(f.completedReasons, completionReason)
	return nil
}

type fakeSyncer struct {
	sourceType string
	buildJobFn func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error)
	executeFn  func(ctx context.Context, job *db.SyncJob) (*Result, error)
}

type fakeSeenStore struct {
	marked [][]string
}

type fakeEnqueuer struct{}

func (f *fakeEnqueuer) EnqueueSync(ctx context.Context, queueName, taskType string, payload []byte, maxRetry int, timeout time.Duration) error {
	return nil
}

func (f *fakeEnqueuer) EnqueueFirstPass(ctx context.Context, artifactID string, payload []byte) error {
	return nil
}

func (f *fakeSyncer) SourceType() string { return f.sourceType }
func (f *fakeSyncer) HeadQueue() string  { return "sync_head:" + f.sourceType }
func (f *fakeSyncer) PageQueue() string  { return "sync:" + f.sourceType }

func (f *fakeSyncer) BuildJob(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	if f.buildJobFn != nil {
		return f.buildJobFn(source, cycle)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeSyncer) Execute(ctx context.Context, job *db.SyncJob) (*Result, error) {
	if f.executeFn != nil {
		return f.executeFn(ctx, job)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeSeenStore) Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (f *fakeSeenStore) MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error {
	cp := append([]string(nil), externalIDs...)
	f.marked = append(f.marked, cp)
	return nil
}

func TestQueueClaimBatchBuildsJobsFromSyncer(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{
		claimed: []*db.Source{{
			ID:   "source-1",
			Type: "reddit",
			KgID: "kg-1",
			Config: map[string]any{
				"subreddit": "golang",
			},
		}},
	}
	wantJob := &db.ScheduledJob{
		ID:       "job-1",
		Type:     "sync:reddit",
		Priority: 5,
		MaxRetry: 3,
		Timeout:  time.Minute,
	}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if source.ID != "source-1" {
				t.Fatalf("BuildJob got source ID %q, want source-1", source.ID)
			}
			if cycle == nil || cycle.Kind != CycleKindRefresh {
				t.Fatalf("cycle = %#v, want refresh cycle", cycle)
			}
			return wantJob, nil
		},
	}

	q := NewQueue(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore(), syncer)

	jobs, err := q.ClaimBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if jobs[0] != wantJob {
		t.Fatalf("ClaimBatch returned wrong job pointer")
	}
	if len(repo.markFailedIDs) != 0 {
		t.Fatalf("MarkSyncFailed called unexpectedly: %v", repo.markFailedIDs)
	}
}

func TestQueueClaimBatchMarksUnsupportedSourcesFailed(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{
		claimed: []*db.Source{{
			ID:   "source-unsupported",
			Type: "hackernews",
		}},
	}
	q := NewQueue(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore())

	jobs, err := q.ClaimBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("ClaimBatch returned %d jobs, want 0", len(jobs))
	}
	if len(repo.markFailedIDs) != 1 || repo.markFailedIDs[0] != "source-unsupported" {
		t.Fatalf("MarkSyncFailed calls = %v, want [source-unsupported]", repo.markFailedIDs)
	}
}

func TestQueueClaimBatchMarksBuildJobFailuresFailed(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{
		claimed: []*db.Source{{
			ID:   "source-bad",
			Type: "reddit",
		}},
	}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			return nil, errors.New("bad config")
		},
	}
	q := NewQueue(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore(), syncer)

	jobs, err := q.ClaimBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("ClaimBatch returned %d jobs, want 0", len(jobs))
	}
	if len(repo.markFailedIDs) != 1 || repo.markFailedIDs[0] != "source-bad" {
		t.Fatalf("MarkSyncFailed calls = %v, want [source-bad]", repo.markFailedIDs)
	}
}

func TestQueueClaimBatchContinuesExistingActiveCycleWhenRefreshNotDue(t *testing.T) {
	t.Parallel()

	lastRefresh := time.Now().UTC()
	repo := &fakeSourceRepo{
		claimed: []*db.Source{{
			ID:            "source-1",
			Type:          "reddit",
			KgID:          "kg-1",
			SyncInterval:  3600,
			LastRefreshAt: &lastRefresh,
			Config: map[string]any{
				"subreddit": "golang",
			},
		}},
	}
	existing := &db.SyncCycle{
		ID:        "cycle-1",
		SourceID:  "source-1",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: time.Now().UTC().Add(-time.Hour),
	}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"source-1": {existing},
		},
	}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if cycle == nil || cycle.ID != "cycle-1" {
				t.Fatalf("cycle = %#v, want existing active cycle", cycle)
			}
			return &db.ScheduledJob{ID: "job-1", Type: "sync:reddit"}, nil
		},
	}

	q := NewQueue(&fakeEnqueuer{}, repo, cycles, NewNoopSeenStore(), syncer)

	jobs, err := q.ClaimBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if len(cycles.created) != 0 {
		t.Fatalf("Create called unexpectedly: %v", cycles.created)
	}
	if len(repo.releasedIDs) != 0 {
		t.Fatalf("Release called unexpectedly: %v", repo.releasedIDs)
	}
}

func TestQueueMarksAcceptedArtifactsSeenAfterEnqueue(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Artifacts: []*RawArtifact{
					{ExternalID: "a-1", Type: "post"},
					{ExternalID: "a-2", Type: "post"},
				},
				HasMore:       false,
				SourceUpdates: map[string]any{},
				CursorUpdates: map[string]any{},
			}, nil
		},
	}

	q := NewQueue(&fakeEnqueuer{}, repo, cycles, seen, syncer)
	handler := q.makeHandler(syncer)

	payload, err := json.Marshal(db.SyncJob{
		CycleID:    "cycle-1",
		SourceID:   "source-1",
		KgID:       "kg-1",
		SourceType: "reddit",
		JobType:    "fetch_posts",
		Payload:    map[string]any{"after": ""},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := handler(context.Background(), asynq.NewTask("sync:reddit", payload)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if len(seen.marked) != 1 {
		t.Fatalf("MarkSeen calls = %d, want 1", len(seen.marked))
	}
	if len(seen.marked[0]) != 2 || seen.marked[0][0] != "a-1" || seen.marked[0][1] != "a-2" {
		t.Fatalf("marked ids = %v, want [a-1 a-2]", seen.marked[0])
	}
	if len(repo.markSyncedIDs) != 1 || repo.markSyncedIDs[0] != "source-1" {
		t.Fatalf("MarkSynced calls = %v, want [source-1]", repo.markSyncedIDs)
	}
	if len(cycles.completedReasons) != 1 || cycles.completedReasons[0] != CompletionReasonExhausted {
		t.Fatalf("completion reasons = %v, want [%s]", cycles.completedReasons, CompletionReasonExhausted)
	}
}
