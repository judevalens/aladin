package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
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
	log := slog.With("component", "sync_orchestrator")
	sources, err := q.sources.ClaimBatch(ctx, limit)
	if err != nil {
		return nil, err
	}
	log.Debug("sync: claimed sources", "count", len(sources), "limit", limit)
	jobs := make([]*db.ScheduledJob, 0, len(sources))
	now := time.Now().UTC()
	for _, src := range sources {
		sourceLog := log.With(
			"source_id", src.ID,
			"source_type", src.Type,
		)
		syncer, ok := q.syncers[src.Type]
		if !ok {
			sourceLog.Warn("sync: no syncer for source type, skipping")
			if err := q.sources.MarkSyncFailed(ctx, src.ID); err != nil {
				sourceLog.Error("sync: mark failed error", "err", err)
			}
			continue
		}

		cycles, err := q.cycles.ListActiveBySource(ctx, src.ID)
		if err != nil {
			if rErr := q.sources.Release(ctx, src.ID); rErr != nil {
				sourceLog.Error("sync: release source after cycle list failure", "err", rErr)
			}
			return nil, fmt.Errorf("claim cycles for source %s: %w", src.ID, err)
		}

		decision := q.arbiter.Decide(src, cycles, now)
		cycleID := ""
		if decision.Cycle != nil {
			cycleID = decision.Cycle.ID
		}
		sourceLog.Debug(
			"sync: arbiter decision",
			"action", decision.Action,
			"reason", decision.Reason,
			"cycle_id", cycleID,
			"active_cycle_count", len(cycles),
		)
		if decision.Action == DecisionSkip {
			if err := q.sources.Release(ctx, src.ID); err != nil {
				sourceLog.Error("sync: release skipped source failed", "reason", decision.Reason, "err", err)
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
					sourceLog.Error("sync: release source after create cycle failure", "err", rErr)
				}
				return nil, fmt.Errorf("create refresh cycle for source %s: %w", src.ID, err)
			}
			sourceLog.Info("sync: created refresh cycle", "cycle_id", cycle.ID)
		}

		job, err := syncer.BuildJob(*src, cycle)
		if err != nil {
			sourceLog.Error("sync: build job failed, marking source failed", "err", err)
			if err := q.sources.MarkSyncFailed(ctx, src.ID); err != nil {
				sourceLog.Error("sync: mark failed error", "err", err)
			}
			continue
		}
		sourceLog.Debug(
			"sync: built job",
			"job_id", job.ID,
			"job_type", job.Type,
			"correlation_id", job.CorrelationID,
			"cycle_id", cycleIDForLog(cycle),
		)
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
	slog.Debug(
		"sync: dispatching job",
		"component", "sync_orchestrator",
		"job_id", job.ID,
		"correlation_id", job.CorrelationID,
		"job_type", job.Type,
		"queue", syncer.HeadQueue(),
		"max_retry", job.MaxRetry,
		"timeout", job.Timeout,
	)
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
			"snapshot_id", job.SnapshotID,
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

		log.Info(
			"sync: syncer returned result",
			"record_count", len(result.Records),
			"has_more", result.HasMore,
			"completion_reason", result.CompletionReason,
			"source_update_keys", mapKeys(result.SourceUpdates),
			"cursor_update_keys", mapKeys(result.CursorUpdates),
			"head_boundary_keys", mapKeys(result.HeadBoundary),
		)

		// Enqueue records into the pipeline.
		// Any enqueue failure is a data loss — fail the whole handler so asynq retries.
		queued := 0
		seenIDs := make([]string, 0, len(result.Records))
		for _, a := range result.Records {
			recordID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(job.SourceID+":"+a.ExternalID)).String()
			payload, err := json.Marshal(pipeline.RecordPayload{
				RecordID:      recordID,
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
				log.Error(
					"sync: marshal record payload failed",
					"external_id", a.ExternalID,
					"record_type", a.Type,
					"err", err,
				)
				return fmt.Errorf("sync: marshal record payload: %w", err)
			}
			if err := q.enqueuer.EnqueueFirstPass(ctx, recordID, payload); err != nil {
				// Enqueue failure — fail the handler so asynq retries the whole page
				if job.SnapshotID == "" {
					_ = q.sources.MarkSyncFailed(ctx, job.SourceID)
				}
				log.Error(
					"sync: enqueue first pass failed",
					"external_id", a.ExternalID,
					"record_id", recordID,
					"record_type", a.Type,
					"err", err,
				)
				return fmt.Errorf("sync: enqueue record %s: %w", a.ExternalID, err)
			}
			log.Debug(
				"sync: enqueued first pass",
				"external_id", a.ExternalID,
				"record_id", recordID,
				"record_type", a.Type,
			)
			queued++
			seenIDs = append(seenIDs, a.ExternalID)
		}
		if err := q.seen.MarkSeen(ctx, job.SourceID, seenIDs); err != nil {
			log.Error("sync: mark seen failed", "seen_count", len(seenIDs), "err", err)
			return fmt.Errorf("sync: mark seen: %w", err)
		}
		log.Info("sync: records accepted for ingestion", "queued", queued, "marked_seen", len(seenIDs), "total", len(result.Records))

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
				log.Info(
					"sync: cycle progress updated",
					"cursor_keys", mapKeys(cursor),
					"head_boundary_keys", mapKeys(result.HeadBoundary),
				)
			}
			if err := q.sources.MarkSyncPage(ctx, job.SourceID, result.SourceUpdates); err != nil {
				log.Error("sync: mark sync page failed", "err", err)
				return fmt.Errorf("sync: mark sync page: %w", err)
			}
			log.Info(
				"sync: page accepted, returning source to scheduler",
				"source_update_keys", mapKeys(result.SourceUpdates),
				"cursor_update_keys", mapKeys(result.CursorUpdates),
			)
		} else {
			reason := result.CompletionReason
			if reason == "" {
				reason = CompletionReasonExhausted
			}
			if job.CycleID != "" {
				if err := q.cycles.Complete(ctx, job.CycleID, result.HeadBoundary, reason); err != nil {
					log.Error("sync: complete cycle failed", "err", err)
					return fmt.Errorf("sync: complete cycle: %w", err)
				}
				log.Info(
					"sync: cycle completed",
					"completion_reason", reason,
					"head_boundary_keys", mapKeys(result.HeadBoundary),
				)
			}
			if err := q.sources.MarkSynced(ctx, job.SourceID, result.SourceUpdates); err != nil {
				log.Error("sync: mark synced failed", "err", err)
				return fmt.Errorf("sync: mark synced: %w", err)
			}
			log.Info("sync: source marked synced", "completion_reason", reason, "source_update_keys", mapKeys(result.SourceUpdates))
		}

		return nil
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func taskType(sourceType string) string      { return "sync:" + sourceType }
func sourceTypeFromTaskType(t string) string { return strings.TrimPrefix(t, "sync:") }

func cycleIDForLog(cycle *db.SyncCycle) string {
	if cycle == nil {
		return ""
	}
	return cycle.ID
}

func mapKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

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
