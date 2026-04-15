package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

// Orchestrator manages sync handler registration, job execution, and scheduler startup.
type Orchestrator struct {
	enqueuer Enqueuer
	sources  db.SourceRepository
	cycles   db.SyncCycleRepository
	seen     SeenStore
	arbiter  Arbiter
	syncers  map[string]Syncer
}

func NewOrchestrator(
	enqueuer Enqueuer,
	sources db.SourceRepository,
	cycles db.SyncCycleRepository,
	seen SeenStore,
	arbiter Arbiter,
	syncers ...Syncer,
) *Orchestrator {
	m := make(map[string]Syncer, len(syncers))
	for _, s := range syncers {
		m[s.SourceType()] = s
	}
	if seen == nil {
		seen = NewNoopSeenStore()
	}
	if arbiter == nil {
		arbiter = NewFreshnessFirstArbiter()
	}
	return &Orchestrator{
		enqueuer: enqueuer,
		sources:  sources,
		cycles:   cycles,
		seen:     seen,
		arbiter:  arbiter,
		syncers:  m,
	}
}

// Start launches the scheduler — polls for due sources and dispatches them.
func (q *Orchestrator) Start(ctx context.Context) {
	go NewScheduler(q, q, defaultBatchSize).Start(ctx)
}

// ClaimBatch implements JobPoller — claims due sources and builds jobs using syncer policy.
// This is where source data meets execution policy: the repo claims rows,
// the syncer decides job shape.
func (q *Orchestrator) ClaimBatch(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
	sources, err := q.sources.ClaimBatch(ctx, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]*db.ScheduledJob, 0, len(sources))
	now := time.Now().UTC()
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

		cycles, err := q.cycles.ListActiveBySource(ctx, src.ID)
		if err != nil {
			if rErr := q.sources.Release(ctx, src.ID); rErr != nil {
				slog.Error("sync: release source after cycle list failure",
					"component", "sync_queue",
					"source_id", src.ID,
					"err", rErr,
				)
			}
			return nil, fmt.Errorf("claim cycles for source %s: %w", src.ID, err)
		}

		decision := q.arbiter.Decide(src, cycles, now)
		if decision.Action == DecisionSkip {
			if err := q.sources.Release(ctx, src.ID); err != nil {
				slog.Error("sync: release skipped source failed",
					"component", "sync_queue",
					"source_id", src.ID,
					"reason", decision.Reason,
					"err", err,
				)
			}
			continue
		}

		cycle := decision.Cycle
		if decision.Action == DecisionCreateRefresh {
			cycle = &db.SyncCycle{
				ID:       uuid.NewString(),
				SourceID: src.ID,
				Kind:     CycleKindRefresh,
				Status:   CycleStatusActive,
			}
			if err := q.cycles.Create(ctx, cycle); err != nil {
				if rErr := q.sources.Release(ctx, src.ID); rErr != nil {
					slog.Error("sync: release source after create cycle failure",
						"component", "sync_queue",
						"source_id", src.ID,
						"err", rErr,
					)
				}
				return nil, fmt.Errorf("create refresh cycle for source %s: %w", src.ID, err)
			}
		}

		job, err := syncer.BuildJob(*src, cycle)
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
func (q *Orchestrator) RegisterHandlers(mux *asynq.ServeMux) {
	for sourceType, syncer := range q.syncers {
		mux.HandleFunc(taskType(sourceType), q.makeHandler(syncer))
		slog.Info("sync: registered handler", "component", "sync_queue", "source_type", sourceType)
	}
}

// Dispatch implements JobDispatcher — enqueues a claimed job to its syncer's queue.
func (q *Orchestrator) Dispatch(ctx context.Context, job *db.ScheduledJob) error {
	syncer, ok := q.syncers[sourceTypeFromTaskType(job.Type)]
	if !ok {
		return fmt.Errorf("dispatch: no syncer for task type %q", job.Type)
	}
	if err := q.enqueuer.EnqueueSync(ctx, syncer.HeadQueue(), job.Type, job.Payload, job.MaxRetry, job.Timeout); err != nil {
		return fmt.Errorf("asynq dispatch: %w", err)
	}
	return nil
}

// Queues returns all queue names with their default weights for asynq server config.
func (q *Orchestrator) Queues() map[string]int {
	m := make(map[string]int, len(q.syncers))
	for _, s := range q.syncers {
		m[s.HeadQueue()] = 6
	}
	return m
}

// ── Handler ───────────────────────────────────────────────────────────────────

func (q *Orchestrator) makeHandler(syncer Syncer) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var job db.SyncJob
		if err := json.Unmarshal(t.Payload(), &job); err != nil {
			return fmt.Errorf("unmarshal job: %w", err)
		}

		log := slog.With(
			"component", "sync_queue",
			"correlation_id", job.CorrelationID,
			"cycle_id", job.CycleID,
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
		if job.CycleID != "" {
			if err := q.cycles.MarkRunning(ctx, job.CycleID); err != nil {
				log.Warn("sync: mark cycle running failed", "err", err)
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

		// Enqueue artifacts into the pipeline.
		// Any enqueue failure is a data loss — fail the whole handler so asynq retries.
		queued := 0
		seenIDs := make([]string, 0, len(result.Artifacts))
		for _, a := range result.Artifacts {
			artifactID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(job.SourceID+":"+a.ExternalID)).String()
			payload, err := json.Marshal(pipeline.ArtifactPayload{
				ArtifactID:    artifactID,
				CorrelationID: job.CorrelationID,
				KgID:          job.KgID,
				SourceID:      job.SourceID,
				ExternalID:    a.ExternalID,
				Type:          a.Type,
				Label:         a.Label,
				Content:       a.Content,
				SourceURL:     a.SourceURL,
				Metadata:      a.Metadata,
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
			seenIDs = append(seenIDs, a.ExternalID)
		}
		if err := q.seen.MarkSeen(ctx, job.SourceID, seenIDs); err != nil {
			return fmt.Errorf("sync: mark seen: %w", err)
		}
		log.Info("sync: artifacts enqueued for ingestion", "queued", queued, "total", len(result.Artifacts))

		// After each page, return the source to the scheduler.
		// HasMore=true → MarkSyncPage (idle, no last_synced_at update, cursor persisted)
		//              → scheduler re-claims immediately alongside other sources (fairness)
		// HasMore=false → MarkSynced (idle, last_synced_at stamped, cycle complete)
		if result.HasMore {
			if job.CycleID != "" {
				cursor := mergeState(job.Payload, result.CursorUpdates)
				if err := q.cycles.UpdateProgress(ctx, job.CycleID, cursor, result.HeadBoundary); err != nil {
					log.Error("sync: update cycle progress failed", "err", err)
					return fmt.Errorf("sync: update cycle progress: %w", err)
				}
			}
			if err := q.sources.MarkSyncPage(ctx, job.SourceID, result.SourceUpdates); err != nil {
				log.Error("sync: mark sync page failed", "err", err)
				return fmt.Errorf("sync: mark sync page: %w", err)
			}
			log.Info("sync: page complete, returning source to scheduler")
		} else {
			if job.CycleID != "" {
				reason := result.CompletionReason
				if reason == "" {
					reason = CompletionReasonExhausted
				}
				if err := q.cycles.Complete(ctx, job.CycleID, result.HeadBoundary, reason); err != nil {
					log.Error("sync: complete cycle failed", "err", err)
					return fmt.Errorf("sync: complete cycle: %w", err)
				}
			}
			if err := q.sources.MarkSynced(ctx, job.SourceID, result.SourceUpdates); err != nil {
				log.Error("sync: mark synced failed", "err", err)
				return fmt.Errorf("sync: mark synced: %w", err)
			}
			log.Info("sync: cycle complete", "source_id", job.SourceID)
		}

		return nil
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func taskType(sourceType string) string      { return "sync:" + sourceType }
func sourceTypeFromTaskType(t string) string { return strings.TrimPrefix(t, "sync:") }

func mergeState(base map[string]any, updates map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(updates))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range updates {
		merged[k] = v
	}
	return merged
}
