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
	enqueuer    Enqueuer
	streams     db.ProviderStreamRepository
	sourceItems db.SourceItemRepository
	cycles      db.SyncCycleRepository
	seen        SeenStore
	arbiter     Arbiter
	syncers     map[string]Syncer
}

func NewOrchestrator(
	enqueuer Enqueuer,
	streams db.ProviderStreamRepository,
	sourceItems db.SourceItemRepository,
	cycles db.SyncCycleRepository,
	seen SeenStore,
	arbiter Arbiter,
	syncers ...Syncer,
) *Orchestrator {
	m := make(map[string]Syncer, len(syncers))
	for _, s := range syncers {
		m[s.Provider()] = s
	}
	if seen == nil {
		seen = NewNoopSeenStore()
	}
	if arbiter == nil {
		arbiter = NewFreshnessFirstArbiter()
	}
	return &Orchestrator{
		enqueuer:    enqueuer,
		streams:     streams,
		sourceItems: sourceItems,
		cycles:      cycles,
		seen:        seen,
		arbiter:     arbiter,
		syncers:     m,
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
			if mErr := q.markSyncFailed(ctx, job.ProviderStreamID); mErr != nil {
				log.Error("sync: mark failed error", "err", mErr)
			}
			return err
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

		// Accept records into durable source-item storage, then enqueue global enrichment.
		// Any enqueue/upsert failure aborts the whole page so progress does not advance.
		queued := 0
		seenIDs := make([]string, 0, len(result.Records))
		for _, a := range result.Records {
			upserted, err := q.sourceItems.Upsert(ctx, sourceItemFromRecord(job, a))
			if err != nil {
				_ = q.markSyncFailed(ctx, job.ProviderStreamID)
				log.Error("sync: upsert source item failed", "external_id", a.ExternalID, "err", err)
				return fmt.Errorf("sync: upsert source item %s: %w", a.ExternalID, err)
			}
			if upserted.Changed {
				payload, err := json.Marshal(pipeline.SourceItemPayload{
					SourceItemID:     upserted.Item.ID,
					CorrelationID:    job.CorrelationID,
					ProviderStreamID: upserted.Item.ProviderStreamID,
					Provider:         upserted.Item.Provider,
					ExternalID:       upserted.Item.ExternalID,
					SourceRevision:   upserted.Item.SourceRevision,
					Type:             upserted.Item.Type,
					Title:            upserted.Item.Title,
					ContentExcerpt:   upserted.Item.ContentExcerpt,
					ContextExcerpt:   upserted.Item.ContextExcerpt,
					SourceURL:        upserted.Item.SourceURL,
					Metadata:         upserted.Item.ProviderMetadata,
				})
				if err != nil {
					return fmt.Errorf("sync: marshal source item payload: %w", err)
				}
				if err := q.enqueuer.EnqueueGlobalFirstPass(ctx, upserted.Item.ID, payload); err != nil {
					_ = q.markSyncFailed(ctx, job.ProviderStreamID)
					log.Error("sync: enqueue global first pass failed", "source_item_id", upserted.Item.ID, "external_id", a.ExternalID, "err", err)
					return fmt.Errorf("sync: enqueue source item %s: %w", a.ExternalID, err)
				}
				queued++
			}
			seenIDs = append(seenIDs, a.ExternalID)
		}
		acceptedAt := time.Now().UTC()
		for _, cycle := range result.NewCycles {
			if cycle == nil {
				continue
			}
			if cycle.ID == "" {
				cycle.ID = uuid.NewString()
			}
			if cycle.ProviderStreamID == "" {
				cycle.ProviderStreamID = job.ProviderStreamID
			}
			if cycle.Status == "" {
				cycle.Status = CycleStatusActive
			}
			if cycle.CreatedAt.IsZero() {
				cycle.CreatedAt = acceptedAt
			}
			if cycle.Kind == CycleKindHydration && cycle.LastHydratedAt == nil {
				cycle.LastHydratedAt = &acceptedAt
			}
			if err := q.cycles.Create(ctx, cycle); err != nil {
				log.Error(
					"sync: create follow-up cycle failed",
					"cycle_kind", cycle.Kind,
					"target_kind", cycle.TargetKind,
					"target_external_id", cycle.TargetExternalID,
					"err", err,
				)
				return fmt.Errorf("sync: create follow-up cycle: %w", err)
			}
			log.Info(
				"sync: follow-up cycle created",
				"cycle_kind", cycle.Kind,
				"target_kind", cycle.TargetKind,
				"target_external_id", cycle.TargetExternalID,
				"last_hydrated_at", cycle.LastHydratedAt,
			)
		}
		if err := q.seen.MarkSeen(ctx, job.ProviderStreamID, seenIDs); err != nil {
			log.Error("sync: mark seen failed", "seen_count", len(seenIDs), "err", err)
			return fmt.Errorf("sync: mark seen: %w", err)
		}
		log.Info("sync: records accepted for ingestion", "queued", queued, "marked_seen", len(seenIDs), "total", len(result.Records))

		// After each page, return the provider stream to the scheduler.
		// HasMore=true → MarkSyncPage (idle, no last_synced_at update, cursor persisted)
		//              → scheduler re-claims immediately alongside other streams (fairness)
		// HasMore=false → MarkSynced (idle, last_synced_at stamped, cycle complete)
		if result.HasMore {
			if job.CycleID != "" {
				cursor := mergeState(job.Payload, result.CursorUpdates)
				var lastHydratedAt *time.Time
				if selectedCycleKind(job) == CycleKindHydration {
					lastHydratedAt = &acceptedAt
				}
				if err := q.cycles.UpdateProgress(ctx, job.CycleID, cursor, result.HeadBoundary, lastHydratedAt); err != nil {
					log.Error("sync: update cycle progress failed", "err", err)
					return fmt.Errorf("sync: update cycle progress: %w", err)
				}
				log.Info(
					"sync: cycle progress updated",
					"cursor_keys", mapKeys(cursor),
					"head_boundary_keys", mapKeys(result.HeadBoundary),
				)
			}
			if err := q.markSyncPage(ctx, job.ProviderStreamID, result.ProviderStreamUpdates); err != nil {
				log.Error("sync: mark sync page failed", "err", err)
				return fmt.Errorf("sync: mark sync page: %w", err)
			}
			log.Info(
				"sync: page accepted, returning provider stream to scheduler",
				"provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates),
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
			if job.CycleID != "" && selectedCycleKind(job) == CycleKindHydration {
				if err := q.markSyncPage(ctx, job.ProviderStreamID, result.ProviderStreamUpdates); err != nil {
					log.Error("sync: mark hydration page complete failed", "err", err)
					return fmt.Errorf("sync: mark hydration page complete: %w", err)
				}
				log.Info("sync: hydration provider stream turn complete", "completion_reason", reason, "provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates))
			} else {
				if err := q.markSynced(ctx, job.ProviderStreamID, result.ProviderStreamUpdates); err != nil {
					log.Error("sync: mark synced failed", "err", err)
					return fmt.Errorf("sync: mark synced: %w", err)
				}
				log.Info("sync: provider stream marked synced", "completion_reason", reason, "provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates))
			}
		}

		return nil
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

func selectedCycleKind(job db.SyncJob) string {
	if kind, _ := job.Payload["cycle_kind"].(string); kind != "" {
		return kind
	}
	return CycleKindRefresh
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

func sourceItemFromRecord(job db.SyncJob, record *RawRecord) *db.SourceItem {
	itemID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(job.Provider+":"+record.ExternalID)).String()
	title := record.Label
	if title == "" {
		title = record.ExternalID
	}
	return &db.SourceItem{
		ID:               itemID,
		ProviderStreamID: job.ProviderStreamID,
		Provider:         job.Provider,
		ExternalID:       record.ExternalID,
		Type:             record.Type,
		Title:            title,
		SourceURL:        record.SourceURL,
		ContentExcerpt:   record.Content,
		ContextExcerpt:   record.EnrichmentContent,
		SourceRevision:   record.SourceRevision,
		ProviderMetadata: record.Metadata,
	}
}

func (q *Orchestrator) markSyncStarted(ctx context.Context, id string) error {
	return q.streams.MarkSyncStarted(ctx, id)
}

func (q *Orchestrator) markSyncFailed(ctx context.Context, id string) error {
	return q.streams.MarkSyncFailed(ctx, id)
}

func (q *Orchestrator) markSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	return q.streams.MarkSyncPage(ctx, id, configUpdates)
}

func (q *Orchestrator) markSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	return q.streams.MarkSynced(ctx, id, configUpdates)
}
