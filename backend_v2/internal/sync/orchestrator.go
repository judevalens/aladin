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
)

// Orchestrator manages sync handler registration, job execution, and scheduler startup.
type Orchestrator struct {
	enqueuer      Enqueuer
	streams       db.ProviderStreamRepository
	cycles        db.SyncCycleRepository
	arbiter       Arbiter
	resultHandler SyncResultHandler
	syncers       map[string]Syncer
}

func NewOrchestrator(
	enqueuer Enqueuer,
	streams db.ProviderStreamRepository,
	records db.RecordRepository,
	cycles db.SyncCycleRepository,
	seen SeenStore,
	arbiter Arbiter,
	syncers ...Syncer,
) *Orchestrator {
	handler := NewRecordResultHandler(enqueuer, streams, records, cycles, seen)
	return NewOrchestratorWithResultHandler(enqueuer, streams, cycles, arbiter, handler, syncers...)
}

func NewOrchestratorWithResultHandler(
	enqueuer Enqueuer,
	streams db.ProviderStreamRepository,
	cycles db.SyncCycleRepository,
	arbiter Arbiter,
	resultHandler SyncResultHandler,
	syncers ...Syncer,
) *Orchestrator {
	m := make(map[string]Syncer, len(syncers))
	for _, s := range syncers {
		m[s.Provider()] = s
	}
	if arbiter == nil {
		arbiter = NewFreshnessFirstArbiter()
	}
	return &Orchestrator{
		enqueuer:      enqueuer,
		streams:       streams,
		cycles:        cycles,
		arbiter:       arbiter,
		resultHandler: resultHandler,
		syncers:       m,
	}
}

// Start launches the scheduler, which polls for due provider streams and dispatches them.
func (q *Orchestrator) Start(ctx context.Context) {
	go NewScheduler(q, q, defaultBatchSize).Start(ctx)
}

// ClaimBatch implements JobPoller. It claims due provider streams and builds jobs using syncer policy.
// This is where provider stream data meets execution policy: the repo claims rows,
// the syncer decides job shape.
func (q *Orchestrator) ClaimBatch(ctx context.Context, limit int) ([]*db.ScheduledJob, error) {
	log := slog.With("component", "sync_orchestrator")
	streams, err := q.streams.ClaimBatch(ctx, limit)
	if err != nil {
		return nil, err
	}
	log.Debug("sync: claimed provider streams", "count", len(streams), "limit", limit)
	jobs := make([]*db.ScheduledJob, 0, len(streams))
	now := time.Now().UTC()
	for _, stream := range streams {
		job, err := q.buildProviderStreamJob(ctx, log, stream, now)
		if err != nil {
			return nil, err
		}
		if job != nil {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (q *Orchestrator) buildProviderStreamJob(ctx context.Context, log *slog.Logger, stream *db.ProviderStream, now time.Time) (*db.ScheduledJob, error) {
	streamLog := log.With(
		"provider_stream_id", stream.ID,
		"provider", stream.Provider,
		"stream_kind", stream.StreamKind,
		"stream_key", stream.StreamKey,
	)
	syncer, ok := q.syncerForProviderStream(ctx, streamLog, stream)
	if !ok {
		return nil, nil
	}
	cycles, err := q.cycles.ListActiveByProviderStream(ctx, stream.ID)
	if err != nil {
		if rErr := q.streams.Release(ctx, stream.ID); rErr != nil {
			streamLog.Error("sync: release stream after cycle list failure", "err", rErr)
		}
		return nil, fmt.Errorf("claim cycles for provider stream %s: %w", stream.ID, err)
	}
	cycle, skip, err := q.selectProviderStreamCycle(ctx, streamLog, stream, cycles, now)
	if err != nil || skip {
		return nil, err
	}
	job, err := syncer.BuildJob(*stream, cycle)
	if err != nil {
		streamLog.Error("sync: build job failed, marking stream failed", "err", err)
		if mErr := q.streams.MarkSyncFailed(ctx, stream.ID); mErr != nil {
			streamLog.Error("sync: mark stream failed error", "err", mErr)
		}
		return nil, nil
	}
	logBuiltJob(streamLog, job, cycle)
	return job, nil
}

func (q *Orchestrator) syncerForProviderStream(ctx context.Context, log *slog.Logger, stream *db.ProviderStream) (Syncer, bool) {
	syncer, ok := q.syncers[stream.Provider]
	if ok {
		return syncer, true
	}
	log.Warn("sync: no syncer for provider, skipping")
	if err := q.streams.MarkSyncFailed(ctx, stream.ID); err != nil {
		log.Error("sync: mark stream failed error", "err", err)
	}
	return nil, false
}

func (q *Orchestrator) selectProviderStreamCycle(
	ctx context.Context,
	log *slog.Logger,
	stream *db.ProviderStream,
	cycles []*db.SyncCycle,
	now time.Time,
) (*db.SyncCycle, bool, error) {
	decision := q.arbiter.Decide(stream, cycles, now)
	logArbiterDecision(log, decision, len(cycles))
	if decision.Action == DecisionSkip {
		if err := q.streams.Release(ctx, stream.ID); err != nil {
			log.Error("sync: release skipped stream failed", "reason", decision.Reason, "err", err)
		}
		return nil, true, nil
	}
	if decision.Action != DecisionCreateRefresh {
		return decision.Cycle, false, nil
	}
	cycle := &db.SyncCycle{
		ID:               uuid.NewString(),
		ProviderStreamID: stream.ID,
		Kind:             CycleKindRefresh,
		Status:           CycleStatusActive,
	}
	if err := q.cycles.Create(ctx, cycle); err != nil {
		if rErr := q.streams.Release(ctx, stream.ID); rErr != nil {
			log.Error("sync: release stream after create cycle failure", "err", rErr)
		}
		return nil, false, fmt.Errorf("create refresh cycle for provider stream %s: %w", stream.ID, err)
	}
	log.Info("sync: created refresh cycle", "cycle_id", cycle.ID)
	return cycle, false, nil
}

func logArbiterDecision(log *slog.Logger, decision Decision, activeCycleCount int) {
	cycleID := ""
	if decision.Cycle != nil {
		cycleID = decision.Cycle.ID
	}
	log.Debug(
		"sync: arbiter decision",
		"action", decision.Action,
		"reason", decision.Reason,
		"cycle_id", cycleID,
		"active_cycle_count", activeCycleCount,
	)
}

func logBuiltJob(log *slog.Logger, job *db.ScheduledJob, cycle *db.SyncCycle) {
	log.Debug(
		"sync: built job",
		"job_id", job.ID,
		"job_type", job.Type,
		"correlation_id", job.CorrelationID,
		"cycle_id", cycleIDForLog(cycle),
	)
}

// RegisterHandlers registers a sync handler for each provider on the mux.
func (q *Orchestrator) RegisterHandlers(mux *asynq.ServeMux) {
	for provider, syncer := range q.syncers {
		mux.HandleFunc(taskType(provider), q.makeHandler(syncer))
		slog.Info("sync: registered handler", "component", "sync_queue", "provider", provider)
	}
}

// Dispatch implements JobDispatcher — enqueues a claimed job to its syncer's queue.
func (q *Orchestrator) Dispatch(ctx context.Context, job *db.ScheduledJob) error {
	syncer, ok := q.syncers[providerFromTaskType(job.Type)]
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
			"provider", job.Provider,
			"job_type", job.JobType,
			"provider_stream_id", job.ProviderStreamID,
		)
		log.Info("sync: executing")

		// Non-fatal; monitoring may be degraded but execution can proceed.
		if err := q.markSyncStarted(ctx, job.ProviderStreamID); err != nil {
			log.Warn("sync: mark started failed; provider stream will appear queued during execution", "err", err)
		}
		if job.CycleID != "" {
			if err := q.cycles.MarkRunning(ctx, job.CycleID); err != nil {
				log.Warn("sync: mark cycle running failed", "err", err)
			}
		}

		result, err := syncer.Execute(ctx, &job)
		if err != nil {
			log.Error("sync: execute failed", "err", err)
			return q.resultHandler.HandleFailure(ctx, job, err)
		}

		log.Info(
			"sync: syncer returned result",
			"record_count", len(result.Records),
			"has_more", result.HasMore,
			"completion_reason", result.CompletionReason,
			"provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates),
			"cursor_update_keys", mapKeys(result.CursorUpdates),
			"head_boundary_keys", mapKeys(result.HeadBoundary),
		)

		return q.resultHandler.HandleSuccess(ctx, job, result)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func taskType(provider string) string      { return "sync:" + provider }
func providerFromTaskType(t string) string { return strings.TrimPrefix(t, "sync:") }

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

func (q *Orchestrator) markSyncStarted(ctx context.Context, id string) error {
	return q.streams.MarkSyncStarted(ctx, id)
}
