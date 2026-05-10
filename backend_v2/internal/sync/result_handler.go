package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

// SyncResultHandler owns provider-specific result acceptance after a syncer has
// executed. The orchestrator should only dispatch, execute, and delegate here.
type SyncResultHandler interface {
	HandleSuccess(ctx context.Context, job db.SyncJob, result *Result) error
	HandleFailure(ctx context.Context, job db.SyncJob, executeErr error) error
}

// RecordResultHandler is the default provider-stream result handler. It
// turns syncer records into canonical records, enqueues global enrichment, and only
// advances stream/cycle progress after that durable handoff succeeds.
type RecordResultHandler struct {
	enqueuer Enqueuer
	streams  db.ProviderStreamRepository
	records  db.RecordRepository
	cycles   db.SyncCycleRepository
	seen     SeenStore
}

func NewRecordResultHandler(
	enqueuer Enqueuer,
	streams db.ProviderStreamRepository,
	records db.RecordRepository,
	cycles db.SyncCycleRepository,
	seen SeenStore,
) *RecordResultHandler {
	if seen == nil {
		seen = NewNoopSeenStore()
	}
	return &RecordResultHandler{
		enqueuer: enqueuer,
		streams:  streams,
		records:  records,
		cycles:   cycles,
		seen:     seen,
	}
}

func (h *RecordResultHandler) HandleFailure(ctx context.Context, job db.SyncJob, executeErr error) error {
	log := syncResultLog(job)
	if mErr := h.streams.MarkSyncFailed(ctx, job.ProviderStreamID); mErr != nil {
		log.Error("sync: mark failed error", "err", mErr)
	}
	return executeErr
}

func (h *RecordResultHandler) HandleSuccess(ctx context.Context, job db.SyncJob, result *Result) error {
	log := syncResultLog(job)

	queued, seenIDs, err := h.acceptRecords(ctx, log, job, result.Records)
	if err != nil {
		return err
	}

	acceptedAt := time.Now().UTC()
	if err := h.createFollowUpCycles(ctx, log, job, result.NewCycles, acceptedAt); err != nil {
		return err
	}

	if err := h.seen.MarkSeen(ctx, job.ProviderStreamID, seenIDs); err != nil {
		log.Error("sync: mark seen failed", "seen_count", len(seenIDs), "err", err)
		return fmt.Errorf("sync: mark seen: %w", err)
	}
	log.Info("sync: records accepted for ingestion", "queued", queued, "marked_seen", len(seenIDs), "total", len(result.Records))

	if result.HasMore {
		return h.handlePageProgress(ctx, log, job, result, acceptedAt)
	}
	return h.handleCompletion(ctx, log, job, result)
}

func (h *RecordResultHandler) acceptRecords(
	ctx context.Context,
	log *slog.Logger,
	job db.SyncJob,
	records []*RawRecord,
) (int, []string, error) {
	queued := 0
	seenIDs := make([]string, 0, len(records))
	for _, record := range records {
		upserted, err := h.records.UpsertCanonical(ctx, canonicalRecordFromRaw(job, record))
		if err != nil {
			_ = h.streams.MarkSyncFailed(ctx, job.ProviderStreamID)
			log.Error("sync: upsert canonical record failed", "external_id", record.ExternalID, "err", err)
			return queued, seenIDs, fmt.Errorf("sync: upsert canonical record %s: %w", record.ExternalID, err)
		}
		if upserted.Changed {
			payload, err := json.Marshal(pipeline.GlobalRecordPayload{
				RecordID:         upserted.Record.ID,
				CorrelationID:    job.CorrelationID,
				ProviderStreamID: upserted.Record.ProviderStreamID,
				Provider:         upserted.Record.Provider,
				ExternalID:       upserted.Record.ExternalID,
				SourceRevision:   upserted.Record.SourceRevision,
				Type:             upserted.Record.Type,
				Title:            upserted.Record.Title,
				ContentExcerpt:   upserted.Record.ContentExcerpt,
				ContextExcerpt:   upserted.Record.ContextExcerpt,
				SourceURL:        upserted.Record.SourceURL,
				Metadata:         upserted.Record.ProviderMetadata,
			})
			if err != nil {
				return queued, seenIDs, fmt.Errorf("sync: marshal global record payload: %w", err)
			}
			if err := h.enqueuer.EnqueueGlobalFirstPass(ctx, upserted.Record.ID, payload); err != nil {
				_ = h.streams.MarkSyncFailed(ctx, job.ProviderStreamID)
				log.Error("sync: enqueue global first pass failed", "record_id", upserted.Record.ID, "external_id", record.ExternalID, "err", err)
				return queued, seenIDs, fmt.Errorf("sync: enqueue canonical record %s: %w", record.ExternalID, err)
			}
			queued++
		}
		seenIDs = append(seenIDs, record.ExternalID)
	}
	return queued, seenIDs, nil
}

func (h *RecordResultHandler) createFollowUpCycles(
	ctx context.Context,
	log *slog.Logger,
	job db.SyncJob,
	cycles []*db.SyncCycle,
	acceptedAt time.Time,
) error {
	for _, cycle := range cycles {
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
		if err := h.cycles.Create(ctx, cycle); err != nil {
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
	return nil
}

func (h *RecordResultHandler) handlePageProgress(
	ctx context.Context,
	log *slog.Logger,
	job db.SyncJob,
	result *Result,
	acceptedAt time.Time,
) error {
	if job.CycleID != "" {
		cursor := mergeState(job.Payload, result.CursorUpdates)
		var lastHydratedAt *time.Time
		if selectedCycleKind(job) == CycleKindHydration {
			lastHydratedAt = &acceptedAt
		}
		if err := h.cycles.UpdateProgress(ctx, job.CycleID, cursor, result.HeadBoundary, lastHydratedAt); err != nil {
			log.Error("sync: update cycle progress failed", "err", err)
			return fmt.Errorf("sync: update cycle progress: %w", err)
		}
		log.Info(
			"sync: cycle progress updated",
			"cursor_keys", mapKeys(cursor),
			"head_boundary_keys", mapKeys(result.HeadBoundary),
		)
	}
	if err := h.streams.MarkSyncPage(ctx, job.ProviderStreamID, result.ProviderStreamUpdates); err != nil {
		log.Error("sync: mark sync page failed", "err", err)
		return fmt.Errorf("sync: mark sync page: %w", err)
	}
	log.Info(
		"sync: page accepted, returning provider stream to scheduler",
		"provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates),
		"cursor_update_keys", mapKeys(result.CursorUpdates),
	)
	return nil
}

func (h *RecordResultHandler) handleCompletion(
	ctx context.Context,
	log *slog.Logger,
	job db.SyncJob,
	result *Result,
) error {
	reason := result.CompletionReason
	if reason == "" {
		reason = CompletionReasonExhausted
	}
	if job.CycleID != "" {
		if err := h.cycles.Complete(ctx, job.CycleID, result.HeadBoundary, reason); err != nil {
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
		if err := h.streams.MarkSyncPage(ctx, job.ProviderStreamID, result.ProviderStreamUpdates); err != nil {
			log.Error("sync: mark hydration page complete failed", "err", err)
			return fmt.Errorf("sync: mark hydration page complete: %w", err)
		}
		log.Info("sync: hydration provider stream turn complete", "completion_reason", reason, "provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates))
		return nil
	}
	if err := h.streams.MarkSynced(ctx, job.ProviderStreamID, result.ProviderStreamUpdates); err != nil {
		log.Error("sync: mark synced failed", "err", err)
		return fmt.Errorf("sync: mark synced: %w", err)
	}
	log.Info("sync: provider stream marked synced", "completion_reason", reason, "provider_stream_update_keys", mapKeys(result.ProviderStreamUpdates))
	return nil
}

func syncResultLog(job db.SyncJob) *slog.Logger {
	return slog.With(
		"component", "sync_queue",
		"correlation_id", job.CorrelationID,
		"cycle_id", job.CycleID,
		"provider", job.Provider,
		"job_type", job.JobType,
		"provider_stream_id", job.ProviderStreamID,
	)
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

func canonicalRecordFromRaw(job db.SyncJob, record *RawRecord) *db.Record {
	recordID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(job.Provider+":"+record.ExternalID)).String()
	title := record.Label
	if title == "" {
		title = record.ExternalID
	}
	return &db.Record{
		ID:               recordID,
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
