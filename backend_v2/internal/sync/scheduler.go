package sync

import (
	"context"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/db"
)

const (
	defaultBatchSize = 50
	defaultBackoff   = 5 * time.Second
)

// JobPoller claims a batch of ready jobs from a backing store.
type JobPoller interface {
	ClaimBatch(ctx context.Context, limit int) ([]*db.ScheduledJob, error)
}

// JobDispatcher enqueues a claimed job for execution.
type JobDispatcher interface {
	Dispatch(ctx context.Context, job *db.ScheduledJob) error
}

// Scheduler is a generic polling engine — polls for jobs, dispatches them.
// It has zero domain knowledge: no sources, no sync types, no asynq.
type Scheduler struct {
	poller     JobPoller
	dispatcher JobDispatcher
	batchSize  int
}

func NewScheduler(poller JobPoller, dispatcher JobDispatcher, batchSize int) *Scheduler {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	return &Scheduler{
		poller:     poller,
		dispatcher: dispatcher,
		batchSize:  batchSize,
	}
}

// Start runs the scheduler loop until ctx is cancelled.
// Drains all due jobs as fast as possible, backs off only when nothing is due.
func (s *Scheduler) Start(ctx context.Context) {
	log := slog.With("component", "source_scheduler")
	log.Info("scheduler: started", "batch_size", s.batchSize)

	for {
		select {
		case <-ctx.Done():
			log.Info("scheduler: stopped")
			return
		default:
		}

		jobs, err := s.poller.ClaimBatch(ctx, s.batchSize)
		if err != nil {
			log.Error("scheduler: claim failed, backing off", "err", err, "backoff", defaultBackoff)
			sleep(ctx, defaultBackoff)
			continue
		}

		if len(jobs) == 0 {
			log.Debug("scheduler: nothing due, backing off", "backoff", defaultBackoff)
			sleep(ctx, defaultBackoff)
			continue
		}

		log.Info("scheduler: claimed jobs", "count", len(jobs))

		for _, job := range jobs {
			if err := s.dispatcher.Dispatch(ctx, job); err != nil {
				log.Error("scheduler: dispatch failed",
					"job_id", job.ID,
					"job_type", job.Type,
					"err", err,
				)
				continue
			}
			log.Info("scheduler: dispatched job", "job_id", job.ID, "job_type", job.Type)
		}

		// Full batch — loop immediately, more may be waiting
		if len(jobs) < s.batchSize {
			sleep(ctx, defaultBackoff)
		}
	}
}

// sleep blocks for d or until ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
