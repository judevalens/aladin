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
	if cycle != nil {
		if cycle.CreatedAt.IsZero() {
			cycle.CreatedAt = time.Now().UTC()
		}
		if f.cyclesBySource == nil {
			f.cyclesBySource = make(map[string][]*db.SyncCycle)
		}
		f.cyclesBySource[cycle.SourceID] = append(f.cyclesBySource[cycle.SourceID], cycle)
	}
	return nil
}

func (f *fakeCycleRepo) MarkRunning(ctx context.Context, id string) error {
	f.runningIDs = append(f.runningIDs, id)
	f.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = CycleStatusRunning
	})
	return nil
}

func (f *fakeCycleRepo) UpdateProgress(ctx context.Context, id string, cursor map[string]any, headBoundary map[string]any) error {
	f.updatedIDs = append(f.updatedIDs, id)
	f.updatedHeads = append(f.updatedHeads, headBoundary)
	f.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = CycleStatusActive
		c.Cursor = cloneMap(cursor)
		if len(c.HeadBoundary) == 0 && len(headBoundary) > 0 {
			c.HeadBoundary = cloneMap(headBoundary)
		}
	})
	return nil
}

func (f *fakeCycleRepo) MarkActive(ctx context.Context, id string) error {
	f.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = CycleStatusActive
	})
	return nil
}

func (f *fakeCycleRepo) Complete(ctx context.Context, id string, headBoundary map[string]any, completionReason string) error {
	f.completedIDs = append(f.completedIDs, id)
	f.completedHeads = append(f.completedHeads, headBoundary)
	f.completedReasons = append(f.completedReasons, completionReason)
	f.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = CycleStatusComplete
		if len(c.HeadBoundary) == 0 && len(headBoundary) > 0 {
			c.HeadBoundary = cloneMap(headBoundary)
		}
		c.CompletionReason = completionReason
		now := time.Now().UTC()
		c.CompletedAt = &now
	})
	return nil
}

func (f *fakeCycleRepo) updateCycle(id string, fn func(*db.SyncCycle)) {
	for _, cycles := range f.cyclesBySource {
		for _, cycle := range cycles {
			if cycle != nil && cycle.ID == id {
				fn(cycle)
			}
		}
	}
}

type fakeSyncer struct {
	sourceType string
	buildJobFn func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error)
	executeFn  func(ctx context.Context, job *db.SyncJob) (*Result, error)
}

type fakeSeenStore struct {
	marked [][]string
}

type fakeArbiter struct {
	decideFn func(source *db.Source, cycles []*db.SyncCycle, now time.Time) Decision
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

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (f *fakeArbiter) Decide(source *db.Source, cycles []*db.SyncCycle, now time.Time) Decision {
	if f != nil && f.decideFn != nil {
		return f.decideFn(source, cycles, now)
	}
	return ChooseCycle(source, cycles, now)
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

	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore(), NewFreshnessFirstArbiter(), syncer)

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
	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore(), NewFreshnessFirstArbiter())

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
	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore(), NewFreshnessFirstArbiter(), syncer)

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

	q := NewOrchestrator(&fakeEnqueuer{}, repo, cycles, NewNoopSeenStore(), NewFreshnessFirstArbiter(), syncer)

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

	q := NewOrchestrator(&fakeEnqueuer{}, repo, cycles, seen, NewFreshnessFirstArbiter(), syncer)
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

func TestQueueClaimBatchUsesInjectedArbiter(t *testing.T) {
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
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if cycle == nil || cycle.ID != "cycle-injected" {
				t.Fatalf("cycle = %#v, want injected cycle", cycle)
			}
			return &db.ScheduledJob{ID: "job-1", Type: "sync:reddit"}, nil
		},
	}
	arbiter := &fakeArbiter{
		decideFn: func(source *db.Source, cycles []*db.SyncCycle, now time.Time) Decision {
			return Decision{
				Action: DecisionRunCycle,
				Cycle: &db.SyncCycle{
					ID:     "cycle-injected",
					Kind:   CycleKindRefresh,
					Status: CycleStatusActive,
				},
				Reason: "test_policy",
			}
		},
	}

	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeCycleRepo{}, NewNoopSeenStore(), arbiter, syncer)

	jobs, err := q.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ClaimBatch returned %d jobs, want 1", len(jobs))
	}
}

func TestOverlappingCyclesNewerCycleCompletesThenOlderCycleResumes(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	lastRefreshOld := now.Add(-2 * time.Hour)
	lastRefreshRecent := now.Add(-time.Minute)

	source := &db.Source{
		ID:            "source-1",
		Type:          "reddit",
		KgID:          "kg-1",
		SyncInterval:  300,
		LastRefreshAt: &lastRefreshOld,
		Config: map[string]any{
			"subreddit": "golang",
		},
	}
	olderCycle := &db.SyncCycle{
		ID:        "cycle-old",
		SourceID:  "source-1",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: now.Add(-119 * time.Minute),
	}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"source-1": {olderCycle},
		},
	}
	repo := &fakeSourceRepo{
		claimed: []*db.Source{source},
	}
	seen := &fakeSeenStore{}
	seenKnown := map[string]bool{"seen-1": true}
	var builtCycleIDs []string
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if cycle == nil {
				t.Fatal("BuildJob cycle = nil")
			}
			builtCycleIDs = append(builtCycleIDs, cycle.ID)
			payload, err := json.Marshal(db.SyncJob{
				CycleID:    cycle.ID,
				SourceID:   source.ID,
				KgID:       source.KgID,
				SourceType: source.Type,
				JobType:    "fetch_posts",
				Payload:    map[string]any{"after": ""},
			})
			if err != nil {
				return nil, err
			}
			return &db.ScheduledJob{
				ID:      cycle.ID,
				Type:    "sync:reddit",
				Payload: payload,
			}, nil
		},
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			switch job.CycleID {
			case olderCycle.ID:
				return &Result{
					Artifacts: []*RawArtifact{
						{ExternalID: "old-1", Type: "post"},
					},
					HasMore:          false,
					CompletionReason: CompletionReasonExhausted,
					SourceUpdates:    map[string]any{},
					CursorUpdates:    map[string]any{},
				}, nil
			default:
				known := make(map[string]bool, len(seenKnown))
				for k, v := range seenKnown {
					known[k] = v
				}
				page := []string{"new-2", "new-1", "seen-1"}
				result := &Result{
					Artifacts:     []*RawArtifact{},
					SourceUpdates: map[string]any{},
					CursorUpdates: map[string]any{},
					HeadBoundary:  map[string]any{},
				}
				for _, id := range page {
					if known[id] {
						result.CompletionReason = CompletionReasonSeenOverlap
						break
					}
					result.Artifacts = append(result.Artifacts, &RawArtifact{ExternalID: id, Type: "post"})
					if len(result.HeadBoundary) == 0 {
						result.HeadBoundary["id"] = id
					}
				}
				return result, nil
			}
		},
	}

	orch := NewOrchestrator(&fakeEnqueuer{}, repo, cycles, seen, NewFreshnessFirstArbiter(), syncer)

	jobs, err := orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("first ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("first ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if len(cycles.created) != 1 {
		t.Fatalf("created cycles = %d, want 1", len(cycles.created))
	}
	newerCycle := cycles.created[0]
	if newerCycle.ID == olderCycle.ID {
		t.Fatalf("newer cycle reused older cycle id %q", newerCycle.ID)
	}

	handler := orch.makeHandler(syncer)
	if err := handler(context.Background(), asynq.NewTask("sync:reddit", jobs[0].Payload)); err != nil {
		t.Fatalf("handler returned error for newer cycle: %v", err)
	}
	if len(cycles.completedIDs) != 1 || cycles.completedIDs[0] != newerCycle.ID {
		t.Fatalf("completed cycle ids = %v, want [%s]", cycles.completedIDs, newerCycle.ID)
	}
	if len(cycles.completedReasons) != 1 || cycles.completedReasons[0] != CompletionReasonSeenOverlap {
		t.Fatalf("completion reasons = %v, want [%s]", cycles.completedReasons, CompletionReasonSeenOverlap)
	}

	cycles.cyclesBySource["source-1"] = []*db.SyncCycle{olderCycle}
	source.LastRefreshAt = &lastRefreshRecent
	repo.markSyncedIDs = nil

	jobs, err = orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("second ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("second ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if len(builtCycleIDs) < 2 || builtCycleIDs[1] != olderCycle.ID {
		t.Fatalf("built cycle ids = %v, want second build for %s", builtCycleIDs, olderCycle.ID)
	}
}

func TestRefreshCycleTakesOverForTwoPagesThenOlderCycleContinues(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	lastRefreshOld := now.Add(-2 * time.Hour)
	lastRefreshRecent := now.Add(-time.Minute)

	source := &db.Source{
		ID:            "source-1",
		Type:          "reddit",
		KgID:          "kg-1",
		SyncInterval:  300,
		LastRefreshAt: &lastRefreshOld,
		Config: map[string]any{
			"subreddit": "golang",
		},
	}
	olderCycle := &db.SyncCycle{
		ID:        "cycle-old",
		SourceID:  "source-1",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: now.Add(-119 * time.Minute),
	}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"source-1": {olderCycle},
		},
	}
	repo := &fakeSourceRepo{
		claimed: []*db.Source{source},
	}
	seen := &fakeSeenStore{}
	var builtCycleIDs []string

	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			builtCycleIDs = append(builtCycleIDs, cycle.ID)
			payload, err := json.Marshal(db.SyncJob{
				CycleID:    cycle.ID,
				SourceID:   source.ID,
				KgID:       source.KgID,
				SourceType: source.Type,
				JobType:    "fetch_posts",
				Payload:    cloneMap(cycle.Cursor),
			})
			if err != nil {
				return nil, err
			}
			return &db.ScheduledJob{
				ID:      cycle.ID,
				Type:    "sync:reddit",
				Payload: payload,
			}, nil
		},
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			switch job.CycleID {
			case olderCycle.ID:
				return &Result{
					Artifacts: []*RawArtifact{
						{ExternalID: "old-1", Type: "post"},
					},
					HasMore:          false,
					CompletionReason: CompletionReasonExhausted,
					SourceUpdates:    map[string]any{},
					CursorUpdates:    map[string]any{},
				}, nil
			default:
				after, _ := job.Payload["after"].(string)
				if after == "" {
					return &Result{
						Artifacts: []*RawArtifact{
							{ExternalID: "new-3", Type: "post"},
							{ExternalID: "new-2", Type: "post"},
						},
						HasMore:       true,
						SourceUpdates: map[string]any{},
						CursorUpdates: map[string]any{"after": "page-2"},
						HeadBoundary:  map[string]any{"id": "new-3"},
					}, nil
				}
				if after == "page-2" {
					return &Result{
						Artifacts: []*RawArtifact{
							{ExternalID: "new-1", Type: "post"},
						},
						HasMore:          false,
						CompletionReason: CompletionReasonSeenOverlap,
						SourceUpdates:    map[string]any{},
						CursorUpdates:    map[string]any{},
					}, nil
				}
				return nil, errors.New("unexpected cursor state")
			}
		},
	}

	orch := NewOrchestrator(&fakeEnqueuer{}, repo, cycles, seen, NewFreshnessFirstArbiter(), syncer)

	jobs, err := orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("first ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("first ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if len(cycles.created) != 1 {
		t.Fatalf("created cycles = %d, want 1", len(cycles.created))
	}
	newerCycle := cycles.created[0]

	handler := orch.makeHandler(syncer)
	if err := handler(context.Background(), asynq.NewTask("sync:reddit", jobs[0].Payload)); err != nil {
		t.Fatalf("handler returned error for newer first page: %v", err)
	}
	if len(cycles.updatedIDs) != 1 || cycles.updatedIDs[0] != newerCycle.ID {
		t.Fatalf("updated cycle ids = %v, want [%s]", cycles.updatedIDs, newerCycle.ID)
	}

	jobs, err = orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("second ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("second ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if len(builtCycleIDs) < 2 || builtCycleIDs[1] != newerCycle.ID {
		t.Fatalf("built cycle ids = %v, want second build for newer cycle %s", builtCycleIDs, newerCycle.ID)
	}

	if err := handler(context.Background(), asynq.NewTask("sync:reddit", jobs[0].Payload)); err != nil {
		t.Fatalf("handler returned error for newer second page: %v", err)
	}
	if len(cycles.completedIDs) == 0 || cycles.completedIDs[0] != newerCycle.ID {
		t.Fatalf("completed cycle ids = %v, want first completion for %s", cycles.completedIDs, newerCycle.ID)
	}
	if len(cycles.completedReasons) == 0 || cycles.completedReasons[0] != CompletionReasonSeenOverlap {
		t.Fatalf("completion reasons = %v, want first reason %s", cycles.completedReasons, CompletionReasonSeenOverlap)
	}

	source.LastRefreshAt = &lastRefreshRecent

	jobs, err = orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("third ClaimBatch returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("third ClaimBatch returned %d jobs, want 1", len(jobs))
	}
	if len(builtCycleIDs) < 3 || builtCycleIDs[2] != olderCycle.ID {
		t.Fatalf("built cycle ids = %v, want third build for older cycle %s", builtCycleIDs, olderCycle.ID)
	}
}
