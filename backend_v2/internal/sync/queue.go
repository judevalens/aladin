package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

const QueueSync = "sync"

// Queue manages sync handler registration, job execution, and scheduler startup.
type Queue struct {
	enqueuer  Enqueuer
	sources   db.SourceRepository
	artifacts db.ArtifactRepository
	syncers   map[string]Syncer
}

func NewQueue(
	enqueuer Enqueuer,
	sources db.SourceRepository,
	artifacts db.ArtifactRepository,
	syncers ...Syncer,
) *Queue {
	m := make(map[string]Syncer, len(syncers))
	for _, s := range syncers {
		m[s.SourceType()] = s
	}
	return &Queue{
		enqueuer:  enqueuer,
		sources:   sources,
		artifacts: artifacts,
		syncers:   m,
	}
}

// Start launches the scheduler — polls for due sources and dispatches them.
func (q *Queue) Start(ctx context.Context) {
	dispatcher := &asynqDispatcher{enqueuer: q.enqueuer}
	scheduler  := NewScheduler(q, dispatcher, defaultBatchSize)
	go scheduler.Start(ctx)
}

// ClaimBatch implements JobPoller — claims due sources and builds jobs using syncer policy.
// This is where source data meets execution policy: the repo claims rows,
// the syncer decides job shape.
func (q *Queue) ClaimBatch(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
	sources, err := q.sources.ClaimBatch(ctx, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]*db.ScheduledJob, 0, len(sources))
	for _, src := range sources {
		syncer, ok := q.syncers[src.Type]
		if !ok {
			slog.Warn("sync: no syncer for source type, skipping",
				"component", "sync_queue",
				"source_id", src.ID,
				"source_type", src.Type,
			)
			if err := q.sources.MarkSyncFailed(ctx, src.ID); err != nil {
				slog.Error("sync: mark failed error",
					"component", "sync_queue",
					"source_id", src.ID,
					"err", err,
				)
			}
			continue
		}
		job, err := syncer.BuildJob(*src)
		if err != nil {
			slog.Error("sync: build job failed, marking source failed",
				"component", "sync_queue",
				"source_id", src.ID,
				"source_type", src.Type,
				"err", err,
			)
			if err := q.sources.MarkSyncFailed(ctx, src.ID); err != nil {
				slog.Error("sync: mark failed error",
					"component", "sync_queue",
					"source_id", src.ID,
					"err", err,
				)
			}
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// RegisterHandlers registers a sync handler for each source type on the mux.
func (q *Queue) RegisterHandlers(mux *asynq.ServeMux) {
	for sourceType, syncer := range q.syncers {
		mux.HandleFunc(taskType(sourceType), q.makeHandler(syncer))
		slog.Info("sync: registered handler", "component", "sync_queue", "source_type", sourceType)
	}
}

// ── AsynqDispatcher ───────────────────────────────────────────────────────────

// asynqDispatcher implements JobDispatcher using asynq.
type asynqDispatcher struct {
	enqueuer Enqueuer
}

func (d *asynqDispatcher) Dispatch(ctx context.Context, job *db.ScheduledJob) error {
	err := d.enqueuer.EnqueueSync(ctx, job.Type, job.Payload, job.MaxRetry, job.Timeout)
	if err != nil {
		return fmt.Errorf("asynq dispatch: %w", err)
	}
	return nil
}

// ── Handler ───────────────────────────────────────────────────────────────────

func (q *Queue) makeHandler(syncer Syncer) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var job db.SyncJob
		if err := json.Unmarshal(t.Payload(), &job); err != nil {
			return fmt.Errorf("unmarshal job: %w", err)
		}

		log := slog.With(
			"component", "sync_queue",
			"source_type", job.SourceType,
			"job_type", job.JobType,
			"source_id", job.SourceID,
		)
		log.Info("sync: executing")

		// Stamp start time on the root job only (first page, no SnapshotID).
		// Non-fatal — monitoring may be degraded but execution can proceed.
		if job.SnapshotID == "" {
			if err := q.sources.MarkSyncStarted(ctx, job.SourceID); err != nil {
				log.Warn("sync: mark started failed — source will appear queued during execution", "err", err)
			}
		}

		result, err := syncer.Execute(ctx, &job)
		if err != nil {
			log.Error("sync: execute failed", "err", err)
			if job.SnapshotID == "" {
				if mErr := q.sources.MarkSyncFailed(ctx, job.SourceID); mErr != nil {
					log.Error("sync: mark failed error", "err", mErr)
				}
			}
			return err
		}

		log.Info("sync: complete", "artifact_count", len(result.Artifacts))

		// Stop-on-known-post: if any artifact on this page already exists in the DB,
		// we've caught up to previously-synced content — stop pagination.
		if result.NextJob != nil && len(result.Artifacts) > 0 {
			ids := make([]string, len(result.Artifacts))
			for i, a := range result.Artifacts {
				ids[i] = a.ExternalID
			}
			known, err := q.artifacts.ExistsExternal(ctx, job.SourceID, ids)
			if err != nil {
				log.Warn("sync: exists check failed, continuing pagination", "err", err)
			} else if len(known) > 0 {
				log.Info("sync: hit known posts, stopping pagination",
					"known_count", len(known),
					"page_size", len(result.Artifacts),
				)
				result.NextJob = nil
			}
		}

		// Enqueue artifacts into the pipeline.
		// Any enqueue failure is a data loss — fail the whole handler so asynq retries.
		queued := 0
		for _, a := range result.Artifacts {
			artifactID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(job.SourceID+":"+a.ExternalID)).String()
			payload, err := json.Marshal(pipeline.ArtifactPayload{
				ArtifactID: artifactID,
				KgID:       job.KgID,
				SourceID:   job.SourceID,
				ExternalID: a.ExternalID,
				Type:       a.Type,
				Label:      a.Label,
				Content:    a.Content,
				SourceURL:  a.SourceURL,
				Metadata:   a.Metadata,
			})
			if err != nil {
				// Marshal failure is a programming error — fail fast
				if job.SnapshotID == "" {
					_ = q.sources.MarkSyncFailed(ctx, job.SourceID)
				}
				return fmt.Errorf("sync: marshal artifact payload: %w", err)
			}
			if err := q.enqueuer.EnqueueFirstPass(ctx, artifactID, payload); err != nil {
				// Enqueue failure — fail the handler so asynq retries the whole page
				if job.SnapshotID == "" {
					_ = q.sources.MarkSyncFailed(ctx, job.SourceID)
				}
				return fmt.Errorf("sync: enqueue artifact %s: %w", a.ExternalID, err)
			}
			queued++
		}
		log.Info("sync: artifacts enqueued for ingestion", "queued", queued, "total", len(result.Artifacts))

		// Paginate — enqueue next page if available
		if result.NextJob != nil {
			result.NextJob.SnapshotID = job.SnapshotID
			result.NextJob.KgID = job.KgID
			if err := q.enqueue(ctx, result.NextJob); err != nil {
				log.Error("sync: enqueue next page failed, marking source failed", "err", err)
				if mErr := q.sources.MarkSyncFailed(ctx, job.SourceID); mErr != nil {
					log.Error("sync: mark failed error", "err", mErr)
				}
				return err
			}
			log.Info("sync: enqueued next page")
		} else {
			// No next page — sync cycle complete, mark source available.
			// Return the error so asynq retries — source remains in syncing
			// and will be recovered by the watchdog (future work).
			if err := q.sources.MarkSynced(ctx, job.SourceID); err != nil {
				log.Error("sync: mark synced failed", "err", err)
				return fmt.Errorf("sync: mark synced: %w", err)
			}
			log.Info("sync: source marked synced", "source_id", job.SourceID)
		}

		return nil
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (q *Queue) enqueue(ctx context.Context, job *db.SyncJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return q.enqueuer.EnqueueSync(ctx, taskType(job.SourceType), payload, job.MaxAttempts, 2*time.Minute)
}

func taskType(sourceType string) string { return "sync:" + sourceType }
