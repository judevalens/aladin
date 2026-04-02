package sync

import (
	"context"
	"errors"
	"testing"

	"aladin/backend_v2/internal/db"
)

type fakePoller struct {
	claimBatchFn func(ctx context.Context, limit int) ([]*db.ScheduledJob, error)
}

func (f *fakePoller) ClaimBatch(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
	return f.claimBatchFn(ctx, limit)
}

type fakeDispatcher struct {
	dispatchFn func(ctx context.Context, job *db.ScheduledJob) error
}

func (f *fakeDispatcher) Dispatch(ctx context.Context, job *db.ScheduledJob) error {
	return f.dispatchFn(ctx, job)
}

func TestSchedulerDispatchesClaimedJobs(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := []*db.ScheduledJob{
		{ID: "job-1", Type: "sync:reddit"},
		{ID: "job-2", Type: "sync:reddit"},
	}
	claimCalls := 0
	dispatchIDs := make([]string, 0, len(jobs))

	poller := &fakePoller{
		claimBatchFn: func(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
			claimCalls++
			if limit != 2 {
				t.Fatalf("ClaimBatch limit = %d, want 2", limit)
			}
			if claimCalls == 1 {
				return jobs, nil
			}
			return nil, context.Canceled
		},
	}
	dispatcher := &fakeDispatcher{
		dispatchFn: func(ctx context.Context, job *db.ScheduledJob) error {
			dispatchIDs = append(dispatchIDs, job.ID)
			if len(dispatchIDs) == len(jobs) {
				cancel()
			}
			return nil
		},
	}

	NewScheduler(poller, dispatcher, 2).Start(ctx)

	if claimCalls != 1 {
		t.Fatalf("ClaimBatch called %d times, want 1", claimCalls)
	}
	if len(dispatchIDs) != 2 || dispatchIDs[0] != "job-1" || dispatchIDs[1] != "job-2" {
		t.Fatalf("dispatched IDs = %v, want [job-1 job-2]", dispatchIDs)
	}
}

func TestSchedulerContinuesAfterDispatchError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := []*db.ScheduledJob{
		{ID: "job-1", Type: "sync:a"},
		{ID: "job-2", Type: "sync:b"},
	}
	poller := &fakePoller{
		claimBatchFn: func(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
			cancel()
			return jobs, nil
		},
	}

	var dispatched []string
	dispatcher := &fakeDispatcher{
		dispatchFn: func(ctx context.Context, job *db.ScheduledJob) error {
			dispatched = append(dispatched, job.ID)
			if job.ID == "job-1" {
				return errors.New("boom")
			}
			return nil
		},
	}

	NewScheduler(poller, dispatcher, 10).Start(ctx)

	if len(dispatched) != 2 || dispatched[0] != "job-1" || dispatched[1] != "job-2" {
		t.Fatalf("dispatched IDs = %v, want [job-1 job-2]", dispatched)
	}
}

func TestSchedulerStopsQuicklyWhenNothingDue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	claimCalls := 0
	poller := &fakePoller{
		claimBatchFn: func(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
			claimCalls++
			cancel()
			return []*db.ScheduledJob{}, nil
		},
	}
	dispatcher := &fakeDispatcher{
		dispatchFn: func(ctx context.Context, job *db.ScheduledJob) error {
			t.Fatalf("Dispatch called unexpectedly")
			return nil
		},
	}

	NewScheduler(poller, dispatcher, 5).Start(ctx)

	if claimCalls != 1 {
		t.Fatalf("ClaimBatch called %d times, want 1", claimCalls)
	}
}
