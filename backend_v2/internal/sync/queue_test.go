package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
)

type fakeSourceRepo struct {
	claimed          []*db.Source
	claimErr         error
	markFailedIDs    []string
	markStartedIDs   []string
	markSyncedIDs    []string
	markFailedErr    error
	markStartedErr   error
	markSyncedErr    error
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

func (f *fakeSourceRepo) MarkSynced(ctx context.Context, id string) error {
	f.markSyncedIDs = append(f.markSyncedIDs, id)
	return f.markSyncedErr
}

func (f *fakeSourceRepo) MarkSyncFailed(ctx context.Context, id string) error {
	f.markFailedIDs = append(f.markFailedIDs, id)
	return f.markFailedErr
}

type fakeSyncer struct {
	sourceType string
	buildJobFn func(source db.Source) (*db.ScheduledJob, error)
	executeFn  func(ctx context.Context, job *db.SyncJob) (*Result, error)
}

func (f *fakeSyncer) SourceType() string { return f.sourceType }

func (f *fakeSyncer) BuildJob(source db.Source) (*db.ScheduledJob, error) {
	if f.buildJobFn != nil {
		return f.buildJobFn(source)
	}
	return nil, errors.New("not implemented")
}

func (f *fakeSyncer) Execute(ctx context.Context, job *db.SyncJob) (*Result, error) {
	if f.executeFn != nil {
		return f.executeFn(ctx, job)
	}
	return nil, errors.New("not implemented")
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
		buildJobFn: func(source db.Source) (*db.ScheduledJob, error) {
			if source.ID != "source-1" {
				t.Fatalf("BuildJob got source ID %q, want source-1", source.ID)
			}
			return wantJob, nil
		},
	}

	q := NewQueue(nil, repo, syncer)

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
	q := NewQueue(nil, repo)

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
		buildJobFn: func(source db.Source) (*db.ScheduledJob, error) {
			return nil, errors.New("bad config")
		},
	}
	q := NewQueue(nil, repo, syncer)

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
