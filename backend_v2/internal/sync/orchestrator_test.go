package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

type fakeSourceRepo struct {
	claimed           []*db.ProviderStream
	claimErr          error
	markFailedIDs     []string
	markStartedIDs    []string
	markPageIDs       []string
	markPageUpdates   []map[string]any
	markSyncedIDs     []string
	markSyncedUpdates []map[string]any
	releasedIDs       []string
	markFailedErr     error
	markStartedErr    error
	markSyncedErr     error
}

func (f *fakeSourceRepo) GetByID(ctx context.Context, id string) (*db.ProviderStream, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSourceRepo) Ensure(ctx context.Context, stream *db.ProviderStream) (*db.ProviderStream, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSourceRepo) ClaimBatch(ctx context.Context, limit int) ([]*db.ProviderStream, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return f.claimed, nil
}

func (f *fakeSourceRepo) MarkSyncStarted(ctx context.Context, id string) error {
	f.markStartedIDs = append(f.markStartedIDs, id)
	return f.markStartedErr
}

func (f *fakeSourceRepo) Heartbeat(ctx context.Context, id string) error { return nil }

func (f *fakeSourceRepo) MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	f.markPageIDs = append(f.markPageIDs, id)
	f.markPageUpdates = append(f.markPageUpdates, cloneMap(configUpdates))
	return nil
}

func (f *fakeSourceRepo) MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	f.markSyncedIDs = append(f.markSyncedIDs, id)
	f.markSyncedUpdates = append(f.markSyncedUpdates, cloneMap(configUpdates))
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
	createErr        error
	runningIDs       []string
	updatedIDs       []string
	updatedCursors   []map[string]any
	updatedHeads     []map[string]any
	updatedHydrated  []*time.Time
	failedIDs        []string
	failedReasons    []string
	completedIDs     []string
	completedHeads   []map[string]any
	completedReasons []string
}

func (f *fakeCycleRepo) ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*db.SyncCycle, error) {
	if f.cyclesBySource == nil {
		return nil, nil
	}
	return f.cyclesBySource[providerStreamID], nil
}

func (f *fakeCycleRepo) Create(ctx context.Context, cycle *db.SyncCycle) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, cycle)
	if cycle != nil {
		if cycle.CreatedAt.IsZero() {
			cycle.CreatedAt = time.Now().UTC()
		}
		if f.cyclesBySource == nil {
			f.cyclesBySource = make(map[string][]*db.SyncCycle)
		}
		key := cycle.ProviderStreamID
		if key == "" {
			key = cycle.ProviderStreamID
		}
		f.cyclesBySource[key] = append(f.cyclesBySource[key], cycle)
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

func (f *fakeCycleRepo) UpdateProgress(ctx context.Context, id string, cursor map[string]any, headBoundary map[string]any, lastHydratedAt *time.Time) error {
	f.updatedIDs = append(f.updatedIDs, id)
	f.updatedCursors = append(f.updatedCursors, cloneMap(cursor))
	f.updatedHeads = append(f.updatedHeads, headBoundary)
	f.updatedHydrated = append(f.updatedHydrated, lastHydratedAt)
	f.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = CycleStatusActive
		c.Cursor = cloneMap(cursor)
		if lastHydratedAt != nil {
			c.LastHydratedAt = lastHydratedAt
		}
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

func (f *fakeCycleRepo) MarkFailed(ctx context.Context, id string, completionReason string) error {
	f.failedIDs = append(f.failedIDs, id)
	f.failedReasons = append(f.failedReasons, completionReason)
	f.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = CycleStatusFailed
		c.CompletionReason = completionReason
		now := time.Now().UTC()
		c.CompletedAt = &now
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
	buildJobFn func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error)
	executeFn  func(ctx context.Context, job *db.SyncJob) (*Result, error)
}

type fakeSeenStore struct {
	marked  [][]string
	markErr error
}

type fakeArbiter struct {
	decideFn func(stream *db.ProviderStream, cycles []*db.SyncCycle, now time.Time) Decision
}

type fakeEnqueuer struct {
	firstPassCalls  []firstPassCall
	firstPassErr    error
	failFirstPassAt int
	failExternalID  string
}

type fakeRecordRepo struct {
	upserted            []*db.Record
	upsertErr           error
	existingExternalIDs map[string]bool
	records             map[string]*db.Record
	enrichments         []*db.RecordEnrichment
}

type fakeResultHandler struct {
	successJobs    []db.SyncJob
	successResults []*Result
	failureJobs    []db.SyncJob
	failureErrs    []error
	successErr     error
	failureErr     error
}

type firstPassCall struct {
	recordID   string
	payload    []byte
	externalID string
}

func (f *fakeEnqueuer) EnqueueSync(ctx context.Context, queueName, taskType string, payload []byte, maxRetry int, timeout time.Duration) error {
	return nil
}

func (f *fakeEnqueuer) EnqueueGlobalFirstPass(ctx context.Context, recordID string, payload []byte) error {
	return f.enqueueGlobal(recordID, payload)
}

func (f *fakeEnqueuer) enqueueGlobal(recordID string, payload []byte) error {
	call := firstPassCall{recordID: recordID, payload: append([]byte(nil), payload...)}
	var rec pipeline.GlobalRecordPayload
	if err := json.Unmarshal(payload, &rec); err == nil {
		call.externalID = rec.ExternalID
	}
	f.firstPassCalls = append(f.firstPassCalls, call)
	if f.firstPassErr != nil {
		return f.firstPassErr
	}
	if f.failFirstPassAt > 0 && len(f.firstPassCalls) == f.failFirstPassAt {
		return fmt.Errorf("enqueue first pass failed at call %d", f.failFirstPassAt)
	}
	if f.failExternalID != "" && call.externalID == f.failExternalID {
		return fmt.Errorf("enqueue first pass failed for external id %s", f.failExternalID)
	}
	return nil
}

func (f *fakeRecordRepo) SaveComplete(ctx context.Context, a *db.CompletedRecord) error {
	return errors.New("not implemented")
}

func (f *fakeRecordRepo) UpsertCanonical(ctx context.Context, record *db.Record) (*db.RecordUpsertResult, error) {
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	cp := *record
	f.upserted = append(f.upserted, &cp)
	if f.records == nil {
		f.records = make(map[string]*db.Record)
	}
	f.records[record.ID] = &cp
	if f.existingExternalIDs != nil && f.existingExternalIDs[record.ExternalID] {
		return &db.RecordUpsertResult{Record: &cp, Changed: false, Inserted: false, IsStale: true}, nil
	}
	return &db.RecordUpsertResult{Record: &cp, Changed: true, Inserted: true}, nil
}

func (f *fakeRecordRepo) Get(ctx context.Context, id string) (*db.Record, error) {
	if f.records != nil {
		if record := f.records[id]; record != nil {
			return record, nil
		}
	}
	return nil, errors.New("not implemented")
}

func (f *fakeRecordRepo) SaveEnrichment(ctx context.Context, enrichment *db.RecordEnrichment) (bool, error) {
	f.enrichments = append(f.enrichments, enrichment)
	return true, nil
}

func (f *fakeRecordRepo) SaveEmbedding(ctx context.Context, recordID string, sourceRevision int64, vec []float32) (bool, error) {
	return true, nil
}

func (f *fakeRecordRepo) ListStuck(ctx context.Context, olderThanSecs, limit int) ([]*db.Record, error) {
	return nil, nil
}

func (f *fakeRecordRepo) MarkFailed(ctx context.Context, recordID, reason string) error {
	return nil
}

func (f *fakeRecordRepo) ResetForRetry(ctx context.Context, recordID string) (bool, error) {
	return false, nil
}

func (f *fakeResultHandler) HandleSuccess(ctx context.Context, job db.SyncJob, result *Result) error {
	f.successJobs = append(f.successJobs, job)
	f.successResults = append(f.successResults, result)
	return f.successErr
}

func (f *fakeResultHandler) HandleFailure(ctx context.Context, job db.SyncJob, executeErr error) error {
	f.failureJobs = append(f.failureJobs, job)
	f.failureErrs = append(f.failureErrs, executeErr)
	if f.failureErr != nil {
		return f.failureErr
	}
	return executeErr
}

func (f *fakeSyncer) Provider() string  { return f.sourceType }
func (f *fakeSyncer) HeadQueue() string { return "sync_head:" + f.sourceType }
func (f *fakeSyncer) PageQueue() string { return "sync:" + f.sourceType }

func (f *fakeSyncer) BuildJob(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	if f.buildJobFn != nil {
		return f.buildJobFn(stream, cycle)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeSyncer) Execute(ctx context.Context, job *db.SyncJob) (*Result, error) {
	if f.executeFn != nil {
		return f.executeFn(ctx, job)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeSeenStore) Seen(ctx context.Context, providerStreamID string, externalIDs []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (f *fakeSeenStore) MarkSeen(ctx context.Context, providerStreamID string, externalIDs []string) error {
	cp := append([]string(nil), externalIDs...)
	f.marked = append(f.marked, cp)
	return f.markErr
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

func syncTask(t *testing.T, job db.SyncJob) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return asynq.NewTask("sync:"+job.Provider, payload)
}

func (f *fakeArbiter) Decide(stream *db.ProviderStream, cycles []*db.SyncCycle, now time.Time) Decision {
	if f != nil && f.decideFn != nil {
		return f.decideFn(stream, cycles, now)
	}
	return ChooseCycle(stream, cycles, now)
}

func TestHandlerDelegatesSuccessfulSyncResult(t *testing.T) {
	t.Parallel()

	wantResult := &Result{
		Records: []*RawRecord{{
			ExternalID: "post-1",
			Type:       "post",
			Label:      "Post 1",
		}},
	}
	syncer := &fakeSyncer{
		sourceType: "bluesky",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			if job.ProviderStreamID != "stream-1" {
				t.Fatalf("Execute got stream %q, want stream-1", job.ProviderStreamID)
			}
			return wantResult, nil
		},
	}
	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	handler := &fakeResultHandler{}
	q := NewOrchestratorWithResultHandler(&fakeEnqueuer{}, repo, cycles, NewFreshnessFirstArbiter(), handler, syncer)

	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		Provider:         "bluesky",
		ProviderStreamID: "stream-1",
		JobType:          "fetch_posts",
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(handler.successJobs) != 1 {
		t.Fatalf("HandleSuccess calls = %d, want 1", len(handler.successJobs))
	}
	if handler.successJobs[0].ProviderStreamID != "stream-1" {
		t.Fatalf("HandleSuccess job stream = %q, want stream-1", handler.successJobs[0].ProviderStreamID)
	}
	if handler.successResults[0] != wantResult {
		t.Fatalf("HandleSuccess result pointer was not delegated")
	}
	if len(handler.failureJobs) != 0 {
		t.Fatalf("HandleFailure called unexpectedly")
	}
	if len(repo.markStartedIDs) != 1 || repo.markStartedIDs[0] != "stream-1" {
		t.Fatalf("MarkSyncStarted = %v, want [stream-1]", repo.markStartedIDs)
	}
}

func TestHandlerDelegatesSyncFailure(t *testing.T) {
	t.Parallel()

	executeErr := errors.New("provider failed")
	syncer := &fakeSyncer{
		sourceType: "bluesky",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return nil, executeErr
		},
	}
	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	handler := &fakeResultHandler{}
	q := NewOrchestratorWithResultHandler(&fakeEnqueuer{}, repo, cycles, NewFreshnessFirstArbiter(), handler, syncer)

	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		Provider:         "bluesky",
		ProviderStreamID: "stream-1",
		JobType:          "fetch_posts",
	}))
	if !errors.Is(err, executeErr) {
		t.Fatalf("handler error = %v, want execute error", err)
	}
	if len(handler.failureJobs) != 1 {
		t.Fatalf("HandleFailure calls = %d, want 1", len(handler.failureJobs))
	}
	if !errors.Is(handler.failureErrs[0], executeErr) {
		t.Fatalf("HandleFailure err = %v, want execute error", handler.failureErrs[0])
	}
	if len(handler.successJobs) != 0 {
		t.Fatalf("HandleSuccess called unexpectedly")
	}
	if len(repo.markFailedIDs) != 0 {
		t.Fatalf("orchestrator marked stream failed directly: %v", repo.markFailedIDs)
	}
}

func TestAsynqErrorHandlerMarksFinalSyncFailure(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"stream-1": {{
				ID:               "cycle-1",
				ProviderStreamID: "stream-1",
				Status:           CycleStatusRunning,
			}},
		},
	}
	task := syncTask(t, db.SyncJob{
		Provider:         "bluesky",
		ProviderStreamID: "stream-1",
		CycleID:          "cycle-1",
		CorrelationID:    "corr-1",
	})

	handleAsynqTaskError(context.Background(), task, errors.New("provider failed"), 3, 3, true, repo, cycles)

	if len(repo.markFailedIDs) != 1 || repo.markFailedIDs[0] != "stream-1" {
		t.Fatalf("MarkSyncFailed = %v, want [stream-1]", repo.markFailedIDs)
	}
	if len(cycles.failedIDs) != 1 || cycles.failedIDs[0] != "cycle-1" {
		t.Fatalf("MarkFailed cycles = %v, want [cycle-1]", cycles.failedIDs)
	}
	if got := cycles.failedReasons[0]; got != CompletionReasonTaskRetryExhausted {
		t.Fatalf("failed reason = %q, want %q", got, CompletionReasonTaskRetryExhausted)
	}
	if got := cycles.cyclesBySource["stream-1"][0].Status; got != CycleStatusFailed {
		t.Fatalf("cycle status = %q, want failed", got)
	}
}

func TestAsynqErrorHandlerIgnoresNonFinalSyncFailure(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	task := syncTask(t, db.SyncJob{
		Provider:         "bluesky",
		ProviderStreamID: "stream-1",
		CycleID:          "cycle-1",
	})

	handleAsynqTaskError(context.Background(), task, errors.New("provider failed"), 1, 3, true, repo, cycles)

	if len(repo.markFailedIDs) != 0 {
		t.Fatalf("MarkSyncFailed called before final retry: %v", repo.markFailedIDs)
	}
	if len(cycles.failedIDs) != 0 {
		t.Fatalf("cycle MarkFailed called before final retry: %v", cycles.failedIDs)
	}
}

func TestQueueClaimBatchBuildsJobsFromSyncer(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{
		claimed: []*db.ProviderStream{{
			ID:       "source-1",
			Provider: "reddit",
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
		buildJobFn: func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if stream.ID != "source-1" {
				t.Fatalf("BuildJob got stream ID %q, want source-1", stream.ID)
			}
			if cycle == nil || cycle.Kind != CycleKindRefresh {
				t.Fatalf("cycle = %#v, want refresh cycle", cycle)
			}
			return wantJob, nil
		},
	}

	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, &fakeCycleRepo{}, NewNoopSeenStore(), NewFreshnessFirstArbiter(), syncer)

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
		claimed: []*db.ProviderStream{{
			ID:       "source-unsupported",
			Provider: "hackernews",
		}},
	}
	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, &fakeCycleRepo{}, NewNoopSeenStore(), NewFreshnessFirstArbiter())

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
		claimed: []*db.ProviderStream{{
			ID:       "source-bad",
			Provider: "reddit",
		}},
	}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			return nil, errors.New("bad config")
		},
	}
	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, &fakeCycleRepo{}, NewNoopSeenStore(), NewFreshnessFirstArbiter(), syncer)

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
		claimed: []*db.ProviderStream{{
			ID:            "source-1",
			Provider:      "reddit",
			SyncInterval:  3600,
			LastRefreshAt: &lastRefresh,
			Config: map[string]any{
				"subreddit": "golang",
			},
		}},
	}
	existing := &db.SyncCycle{
		ID:               "cycle-1",
		ProviderStreamID: "source-1",
		Kind:             CycleKindRefresh,
		Status:           CycleStatusActive,
		CreatedAt:        time.Now().UTC().Add(-time.Hour),
	}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"source-1": {existing},
		},
	}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if cycle == nil || cycle.ID != "cycle-1" {
				t.Fatalf("cycle = %#v, want existing active cycle", cycle)
			}
			return &db.ScheduledJob{ID: "job-1", Type: "sync:reddit"}, nil
		},
	}

	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, cycles, NewNoopSeenStore(), NewFreshnessFirstArbiter(), syncer)

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

func TestQueueMarksAcceptedRecordsSeenAfterEnqueue(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "a-1", Type: "post"},
					{ExternalID: "a-2", Type: "post"},
				},
				HasMore:               false,
				ProviderStreamUpdates: map[string]any{},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	handler := q.makeHandler(syncer)

	payload, err := json.Marshal(db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
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

func TestQueueEnqueueFailureOnFirstRecordBlocksProgress(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{failFirstPassAt: 1}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "a-1", Type: "post"},
					{ExternalID: "a-2", Type: "post"},
				},
				HasMore:               true,
				ProviderStreamUpdates: map[string]any{"last_seen_id": "a-1"},
				CursorUpdates:         map[string]any{"after": "page-2"},
				HeadBoundary:          map[string]any{"id": "a-1"},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	}))
	if err == nil {
		t.Fatal("handler returned nil, want enqueue error")
	}
	if len(enqueuer.firstPassCalls) != 1 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 1", len(enqueuer.firstPassCalls))
	}
	assertNoAcceptedProgress(t, repo, cycles, seen)
	if len(repo.markFailedIDs) != 1 || repo.markFailedIDs[0] != "source-1" {
		t.Fatalf("MarkSyncFailed calls = %v, want [source-1]", repo.markFailedIDs)
	}
}

func TestQueueEnqueueFailureAfterPartialPageBlocksProgressAndRetryIsStable(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{failFirstPassAt: 2}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "a-1", Type: "post"},
					{ExternalID: "a-2", Type: "post"},
				},
				HasMore:               false,
				CompletionReason:      CompletionReasonExhausted,
				ProviderStreamUpdates: map[string]any{"last_seen_id": "a-1"},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	task := syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	})

	err := q.makeHandler(syncer)(context.Background(), task)
	if err == nil {
		t.Fatal("handler returned nil, want enqueue error")
	}
	firstIDs := queuedRecordIDs(enqueuer.firstPassCalls)
	if len(firstIDs) != 2 {
		t.Fatalf("first run queued IDs = %v, want two attempted IDs", firstIDs)
	}
	assertNoAcceptedProgress(t, repo, cycles, seen)

	enqueuer.failFirstPassAt = 0
	enqueuer.firstPassCalls = nil
	if err := q.makeHandler(syncer)(context.Background(), task); err != nil {
		t.Fatalf("retry handler returned error: %v", err)
	}
	secondIDs := queuedRecordIDs(enqueuer.firstPassCalls)
	if !reflect.DeepEqual(firstIDs, secondIDs) {
		t.Fatalf("queued record IDs changed across retry: first=%v second=%v", firstIDs, secondIDs)
	}
	if len(seen.marked) != 1 || !reflect.DeepEqual(seen.marked[0], []string{"a-1", "a-2"}) {
		t.Fatalf("MarkSeen calls = %v, want [[a-1 a-2]] after retry success", seen.marked)
	}
	if len(repo.markSyncedIDs) != 1 || repo.markSyncedIDs[0] != "source-1" {
		t.Fatalf("MarkSynced calls = %v, want [source-1]", repo.markSyncedIDs)
	}
	if len(cycles.completedIDs) != 1 || cycles.completedIDs[0] != "cycle-1" {
		t.Fatalf("completed cycle ids = %v, want [cycle-1]", cycles.completedIDs)
	}
}

func TestQueueExecuteFailureDoesNotEnqueueOrAdvance(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return nil, errors.New("source api failed")
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	}))
	if err == nil {
		t.Fatal("handler returned nil, want execute error")
	}
	if len(enqueuer.firstPassCalls) != 0 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 0", len(enqueuer.firstPassCalls))
	}
	assertNoAcceptedProgress(t, repo, cycles, seen)
	if len(repo.markFailedIDs) != 1 || repo.markFailedIDs[0] != "source-1" {
		t.Fatalf("MarkSyncFailed calls = %v, want [source-1]", repo.markFailedIDs)
	}
}

func TestQueueExistingRecordSkipsGlobalFirstPassEnqueue(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{}
	items := &fakeRecordRepo{existingExternalIDs: map[string]bool{"bsky-1": true}}
	syncer := &fakeSyncer{
		sourceType: "bluesky",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "bsky-1", Type: "post"},
				},
				HasMore:               false,
				CompletionReason:      CompletionReasonExhausted,
				ProviderStreamUpdates: map[string]any{},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, items, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	if err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "stream-1",
		Provider:         "bluesky",
		JobType:          "search_posts",
		Payload:          map[string]any{},
	})); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(enqueuer.firstPassCalls) != 0 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 0", len(enqueuer.firstPassCalls))
	}
	if len(items.upserted) != 1 {
		t.Fatalf("record upserts = %d, want 1 existence check", len(items.upserted))
	}
	if len(repo.markSyncedIDs) != 1 || repo.markSyncedIDs[0] != "stream-1" {
		t.Fatalf("MarkSynced calls = %v, want [stream-1]", repo.markSyncedIDs)
	}
}

func TestQueueMarkSeenFailureBlocksProgress(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{markErr: errors.New("seen unavailable")}
	enqueuer := &fakeEnqueuer{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "a-1", Type: "post"},
				},
				HasMore:               false,
				CompletionReason:      CompletionReasonExhausted,
				ProviderStreamUpdates: map[string]any{"last_seen_id": "a-1"},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	}))
	if err == nil {
		t.Fatal("handler returned nil, want MarkSeen error")
	}
	if len(enqueuer.firstPassCalls) != 1 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 1", len(enqueuer.firstPassCalls))
	}
	if len(seen.marked) != 1 || !reflect.DeepEqual(seen.marked[0], []string{"a-1"}) {
		t.Fatalf("MarkSeen calls = %v, want [[a-1]]", seen.marked)
	}
	if len(repo.markPageIDs) != 0 || len(repo.markSyncedIDs) != 0 {
		t.Fatalf("source progress advanced despite MarkSeen failure: pages=%v synced=%v", repo.markPageIDs, repo.markSyncedIDs)
	}
	if len(cycles.updatedIDs) != 0 || len(cycles.completedIDs) != 0 {
		t.Fatalf("cycle progress advanced despite MarkSeen failure: updated=%v completed=%v", cycles.updatedIDs, cycles.completedIDs)
	}
}

func TestQueueRetryProducesStablePayloadWithProvenance(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{
						ExternalID: "t3_post-1",
						Type:       "post",
						Label:      "Post title",
						Content:    "transient raw content",
						SourceURL:  "https://reddit.com/r/golang/comments/post_1/title",
						Metadata: map[string]any{
							"platform": "reddit",
							"score":    7,
						},
					},
				},
				HasMore:               false,
				CompletionReason:      CompletionReasonExhausted,
				ProviderStreamUpdates: map[string]any{"last_seen_id": "t3_post-1"},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	task := syncTask(t, db.SyncJob{
		CorrelationID:    "corr-1",
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	})

	if err := q.makeHandler(syncer)(context.Background(), task); err != nil {
		t.Fatalf("first handler returned error: %v", err)
	}
	if err := q.makeHandler(syncer)(context.Background(), task); err != nil {
		t.Fatalf("second handler returned error: %v", err)
	}
	if len(enqueuer.firstPassCalls) != 2 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 2", len(enqueuer.firstPassCalls))
	}
	if enqueuer.firstPassCalls[0].recordID != enqueuer.firstPassCalls[1].recordID {
		t.Fatalf("record IDs changed across retry: %q vs %q", enqueuer.firstPassCalls[0].recordID, enqueuer.firstPassCalls[1].recordID)
	}
	if string(enqueuer.firstPassCalls[0].payload) != string(enqueuer.firstPassCalls[1].payload) {
		t.Fatalf("payload changed across retry:\nfirst=%s\nsecond=%s", enqueuer.firstPassCalls[0].payload, enqueuer.firstPassCalls[1].payload)
	}

	var queued pipeline.GlobalRecordPayload
	if err := json.Unmarshal(enqueuer.firstPassCalls[0].payload, &queued); err != nil {
		t.Fatalf("unmarshal queued payload: %v", err)
	}
	if queued.CorrelationID != "corr-1" ||
		queued.ProviderStreamID != "source-1" ||
		queued.ExternalID != "t3_post-1" ||
		queued.SourceURL != "https://reddit.com/r/golang/comments/post_1/title" {
		t.Fatalf("queued payload missing provenance: %+v", queued)
	}
	if queued.Metadata["platform"] != "reddit" || queued.Metadata["score"] != float64(7) {
		t.Fatalf("queued metadata = %v, want reddit provenance metadata", queued.Metadata)
	}
	if len(repo.markSyncedUpdates) != 2 ||
		!reflect.DeepEqual(repo.markSyncedUpdates[0], repo.markSyncedUpdates[1]) {
		t.Fatalf("source updates changed across retry: %v", repo.markSyncedUpdates)
	}
}

func TestQueueFollowUpCycleCreateFailureBlocksProgress(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{createErr: errors.New("cycle insert failed")}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "t3_post-1", Type: "post", Content: "post"},
				},
				NewCycles: []*db.SyncCycle{{
					Kind:             CycleKindHydration,
					TargetKind:       "reddit_post",
					TargetExternalID: "t3_post-1",
					Cursor:           map[string]any{"post_id": "t3_post-1", "hydration_interval_seconds": 3600},
				}},
				HasMore:               false,
				CompletionReason:      CompletionReasonExhausted,
				ProviderStreamUpdates: map[string]any{"last_seen_id": "t3_post-1"},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	}))
	if err == nil {
		t.Fatal("handler returned nil, want cycle create error")
	}
	if len(enqueuer.firstPassCalls) != 1 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 1", len(enqueuer.firstPassCalls))
	}
	assertNoAcceptedProgress(t, repo, cycles, seen)
}

func TestQueueCreatesHydrationCyclesAfterRecordsAccepted(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{}
	cycles := &fakeCycleRepo{}
	seen := &fakeSeenStore{}
	enqueuer := &fakeEnqueuer{}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		executeFn: func(ctx context.Context, job *db.SyncJob) (*Result, error) {
			return &Result{
				Records: []*RawRecord{
					{ExternalID: "t3_post-1", Type: "post", Content: "post"},
				},
				NewCycles: []*db.SyncCycle{{
					Kind:             CycleKindHydration,
					TargetKind:       "reddit_post",
					TargetExternalID: "t3_post-1",
					Cursor:           map[string]any{"post_id": "t3_post-1", "hydration_interval_seconds": 3600},
				}},
				HasMore:               false,
				CompletionReason:      CompletionReasonExhausted,
				ProviderStreamUpdates: map[string]any{"last_seen_id": "t3_post-1"},
				CursorUpdates:         map[string]any{},
			}, nil
		},
	}

	q := NewOrchestrator(enqueuer, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)
	err := q.makeHandler(syncer)(context.Background(), syncTask(t, db.SyncJob{
		CycleID:          "cycle-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload:          map[string]any{"after": ""},
	}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(enqueuer.firstPassCalls) != 1 {
		t.Fatalf("EnqueueGlobalFirstPass calls = %d, want 1", len(enqueuer.firstPassCalls))
	}
	if len(cycles.created) != 1 {
		t.Fatalf("created cycles = %d, want 1", len(cycles.created))
	}
	created := cycles.created[0]
	if created.ID == "" || created.ProviderStreamID != "source-1" || created.Status != CycleStatusActive {
		t.Fatalf("created cycle missing orchestrator defaults: %#v", created)
	}
	if created.Kind != CycleKindHydration || created.TargetExternalID != "t3_post-1" || created.LastHydratedAt == nil {
		t.Fatalf("created cycle = %#v, want hydration cycle for t3_post-1", created)
	}
	if len(seen.marked) != 1 || !reflect.DeepEqual(seen.marked[0], []string{"t3_post-1"}) {
		t.Fatalf("MarkSeen calls = %v, want [[t3_post-1]]", seen.marked)
	}
}

func assertNoAcceptedProgress(t *testing.T, repo *fakeSourceRepo, cycles *fakeCycleRepo, seen *fakeSeenStore) {
	t.Helper()
	if len(seen.marked) != 0 {
		t.Fatalf("MarkSeen calls = %v, want none", seen.marked)
	}
	if len(repo.markPageIDs) != 0 {
		t.Fatalf("MarkSyncPage calls = %v, want none", repo.markPageIDs)
	}
	if len(repo.markSyncedIDs) != 0 {
		t.Fatalf("MarkSynced calls = %v, want none", repo.markSyncedIDs)
	}
	if len(cycles.updatedIDs) != 0 {
		t.Fatalf("UpdateProgress calls = %v, want none", cycles.updatedIDs)
	}
	if len(cycles.completedIDs) != 0 {
		t.Fatalf("Complete calls = %v, want none", cycles.completedIDs)
	}
	if len(cycles.created) != 0 {
		t.Fatalf("Create calls = %v, want none", cycles.created)
	}
}

func queuedRecordIDs(calls []firstPassCall) []string {
	ids := make([]string, 0, len(calls))
	for _, call := range calls {
		ids = append(ids, call.recordID)
	}
	return ids
}

func TestQueueClaimBatchUsesInjectedArbiter(t *testing.T) {
	t.Parallel()

	repo := &fakeSourceRepo{
		claimed: []*db.ProviderStream{{
			ID:       "source-1",
			Provider: "reddit",
			Config: map[string]any{
				"subreddit": "golang",
			},
		}},
	}
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if cycle == nil || cycle.ID != "cycle-injected" {
				t.Fatalf("cycle = %#v, want injected cycle", cycle)
			}
			return &db.ScheduledJob{ID: "job-1", Type: "sync:reddit"}, nil
		},
	}
	arbiter := &fakeArbiter{
		decideFn: func(stream *db.ProviderStream, cycles []*db.SyncCycle, now time.Time) Decision {
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

	q := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, &fakeCycleRepo{}, NewNoopSeenStore(), arbiter, syncer)

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

	source := &db.ProviderStream{
		ID:            "source-1",
		Provider:      "reddit",
		SyncInterval:  300,
		LastRefreshAt: &lastRefreshOld,
		Config: map[string]any{
			"subreddit": "golang",
		},
	}
	olderCycle := &db.SyncCycle{
		ID:               "cycle-old",
		ProviderStreamID: "source-1",
		Kind:             CycleKindRefresh,
		Status:           CycleStatusActive,
		CreatedAt:        now.Add(-119 * time.Minute),
	}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"source-1": {olderCycle},
		},
	}
	repo := &fakeSourceRepo{
		claimed: []*db.ProviderStream{source},
	}
	seen := &fakeSeenStore{}
	seenKnown := map[string]bool{"seen-1": true}
	var builtCycleIDs []string
	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			if cycle == nil {
				t.Fatal("BuildJob cycle = nil")
			}
			builtCycleIDs = append(builtCycleIDs, cycle.ID)
			payload, err := json.Marshal(db.SyncJob{
				CycleID:          cycle.ID,
				ProviderStreamID: stream.ID,
				Provider:         stream.Provider,
				JobType:          "fetch_posts",
				Payload:          map[string]any{"after": ""},
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
					Records: []*RawRecord{
						{ExternalID: "old-1", Type: "post"},
					},
					HasMore:               false,
					CompletionReason:      CompletionReasonExhausted,
					ProviderStreamUpdates: map[string]any{},
					CursorUpdates:         map[string]any{},
				}, nil
			default:
				known := make(map[string]bool, len(seenKnown))
				for k, v := range seenKnown {
					known[k] = v
				}
				page := []string{"new-2", "new-1", "seen-1"}
				result := &Result{
					Records:               []*RawRecord{},
					ProviderStreamUpdates: map[string]any{},
					CursorUpdates:         map[string]any{},
					HeadBoundary:          map[string]any{},
				}
				for _, id := range page {
					if known[id] {
						result.CompletionReason = CompletionReasonSeenOverlap
						break
					}
					result.Records = append(result.Records, &RawRecord{ExternalID: id, Type: "post"})
					if len(result.HeadBoundary) == 0 {
						result.HeadBoundary["id"] = id
					}
				}
				return result, nil
			}
		},
	}

	orch := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)

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

	source := &db.ProviderStream{
		ID:            "source-1",
		Provider:      "reddit",
		SyncInterval:  300,
		LastRefreshAt: &lastRefreshOld,
		Config: map[string]any{
			"subreddit": "golang",
		},
	}
	olderCycle := &db.SyncCycle{
		ID:               "cycle-old",
		ProviderStreamID: "source-1",
		Kind:             CycleKindRefresh,
		Status:           CycleStatusActive,
		CreatedAt:        now.Add(-119 * time.Minute),
	}
	cycles := &fakeCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			"source-1": {olderCycle},
		},
	}
	repo := &fakeSourceRepo{
		claimed: []*db.ProviderStream{source},
	}
	seen := &fakeSeenStore{}
	var builtCycleIDs []string

	syncer := &fakeSyncer{
		sourceType: "reddit",
		buildJobFn: func(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
			builtCycleIDs = append(builtCycleIDs, cycle.ID)
			payload, err := json.Marshal(db.SyncJob{
				CycleID:          cycle.ID,
				ProviderStreamID: stream.ID,
				Provider:         stream.Provider,
				JobType:          "fetch_posts",
				Payload:          cloneMap(cycle.Cursor),
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
					Records: []*RawRecord{
						{ExternalID: "old-1", Type: "post"},
					},
					HasMore:               false,
					CompletionReason:      CompletionReasonExhausted,
					ProviderStreamUpdates: map[string]any{},
					CursorUpdates:         map[string]any{},
				}, nil
			default:
				after, _ := job.Payload["after"].(string)
				if after == "" {
					return &Result{
						Records: []*RawRecord{
							{ExternalID: "new-3", Type: "post"},
							{ExternalID: "new-2", Type: "post"},
						},
						HasMore:               true,
						ProviderStreamUpdates: map[string]any{},
						CursorUpdates:         map[string]any{"after": "page-2"},
						HeadBoundary:          map[string]any{"id": "new-3"},
					}, nil
				}
				if after == "page-2" {
					return &Result{
						Records: []*RawRecord{
							{ExternalID: "new-1", Type: "post"},
						},
						HasMore:               false,
						CompletionReason:      CompletionReasonSeenOverlap,
						ProviderStreamUpdates: map[string]any{},
						CursorUpdates:         map[string]any{},
					}, nil
				}
				return nil, errors.New("unexpected cursor state")
			}
		},
	}

	orch := NewOrchestrator(&fakeEnqueuer{}, repo, &fakeRecordRepo{}, cycles, seen, NewFreshnessFirstArbiter(), syncer)

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
