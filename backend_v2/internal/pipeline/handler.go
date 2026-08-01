package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"aladin/backend_v2/internal/db"
)

// FullPipelineHandler implements ResultHandler for the record enrichment pipeline:
//
//	global_first_pass → ├─ tenant_match → insights
//	                    └─ resolve_entities → [resolve_low_confidence] → embed (terminal)
//
// resolve_low_confidence (web search for the entities the first pass was unsure about) sits IN the
// chain, not beside it: the record can't advance until every entity — confident and low-confidence
// — is resolved. It's only inserted when a searcher is configured; otherwise resolve_entities
// chains straight to embed. Adding a new pipeline variant means implementing a new ResultHandler
// — no other changes.
type FullPipelineHandler struct {
	enqueuer        Enqueuer
	repo            db.RecordRepository
	matches         db.TenantItemMatchRepository
	insightEnqueuer InsightEnqueuer
	graphEnabled    bool
	lowConfEnabled  bool
}

// WithLowConfidenceSearch inserts the resolve_low_confidence stage after resolve_entities
// (web search → resolve). Off unless a searcher (Tavily key) is configured — then
// resolve_entities chains straight to embed.
func (h *FullPipelineHandler) WithLowConfidenceSearch(enabled bool) *FullPipelineHandler {
	h.lowConfEnabled = enabled
	return h
}

// WithGraphProjection enables enqueuing the Neo4j graph-projection stage after entities.
// Off unless Neo4j is configured (so the stage is never enqueued without a worker to run it).
func (h *FullPipelineHandler) WithGraphProjection(enabled bool) *FullPipelineHandler {
	h.graphEnabled = enabled
	return h
}

func NewFullPipelineHandler(
	enqueuer Enqueuer,
	repo db.RecordRepository,
) *FullPipelineHandler {
	return &FullPipelineHandler{enqueuer: enqueuer, repo: repo}
}

func (h *FullPipelineHandler) WithTenantItemMatches(repo db.TenantItemMatchRepository) *FullPipelineHandler {
	h.matches = repo
	return h
}

type InsightEnqueuer interface {
	EnqueueInsightGeneration(ctx context.Context, kgID string, recordID string, sourceRevision int64, correlationID string, generatorKeys []string) error
}

func (h *FullPipelineHandler) WithInsightEnqueuer(enqueuer InsightEnqueuer) *FullPipelineHandler {
	h.insightEnqueuer = enqueuer
	return h
}

func (h *FullPipelineHandler) OnDone(ctx context.Context, result Result) error {
	log := slog.With(
		"component", "orchestrator",
		"record_id", result.RecordID,
		"correlation_id", result.CorrelationID,
		"kg_id", result.KgID,
		"result_type", result.Type,
	)

	if result.Err != nil {
		return h.handleError(ctx, log, result)
	}

	switch result.Type {
	case ResultGlobalFirstPassDone:
		log.Info("orchestrator: global enrichment complete, persisting", "checkpoint", "enriched")
		return h.persistGlobalRecordEnrichment(ctx, log, result.Payload)

	case ResultTenantMatchDone:
		log.Info("orchestrator: tenant matching complete")
		return h.enqueueInsightTriggers(ctx, log, result.Payload)

	case ResultResolveEntitiesDone:
		// Entities are resolved. When web search is configured, resolve the low-confidence
		// entities first so the entity set is complete before the record advances; otherwise
		// chain straight on. Payloads share a shape, so forward directly.
		if h.lowConfEnabled {
			log.Info("orchestrator: entity resolution complete, routing to low-confidence search", "checkpoint", "entities_resolved")
			return h.enqueue(ctx, TaskResolveLowConfidence, result.RecordID, result.Payload)
		}
		return h.afterEntities(ctx, log, result)

	case ResultEmbedDone:
		// Terminal: the worker has written the vector and marked the record complete.
		log.Info("orchestrator: record complete", "checkpoint", "completed")
		return nil

	case ResultGraphProjectDone:
		// Terminal: the record's entity slice is projected into Neo4j.
		log.Info("orchestrator: graph projection complete", "checkpoint", "graph_projected")
		return nil

	case ResultResolveLowConfidenceDone:
		// Low-confidence entities are now resolved into the layer too — the entity set is
		// complete, so the record can advance. Payload shape is shared.
		log.Info("orchestrator: low-confidence resolution complete", "checkpoint", "low_confidence_resolved")
		return h.afterEntities(ctx, log, result)

	default:
		return fmt.Errorf("orchestrator: unknown result type %q", result.Type)
	}
}

// afterEntities is the tail of the entity chain, shared by the plain and low-confidence
// routes: embed (terminal) and, when Neo4j is configured, a parallel graph projection —
// it only needs the entities just written, not the embedding.
func (h *FullPipelineHandler) afterEntities(ctx context.Context, log *slog.Logger, result Result) error {
	log.Info("orchestrator: entity layer written, routing to embed", "checkpoint", "entities_written")
	if h.graphEnabled {
		if err := h.enqueue(ctx, TaskGraphProject, result.RecordID, result.Payload); err != nil {
			return err
		}
	}
	return h.enqueue(ctx, TaskEmbed, result.RecordID, result.Payload)
}

func (h *FullPipelineHandler) enqueueInsightTriggers(ctx context.Context, log *slog.Logger, payload []byte) error {
	if h.insightEnqueuer == nil {
		return nil
	}
	var p TenantMatchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("tenant match insights: unmarshal: %w", err)
	}
	for _, trigger := range p.InsightTriggers {
		if trigger.KgID == "" {
			continue
		}
		recordID := firstNonEmpty(trigger.RecordID, p.RecordID)
		sourceRevision := trigger.SourceRevision
		if sourceRevision == 0 {
			sourceRevision = p.SourceRevision
		}
		correlationID := firstNonEmpty(trigger.CorrelationID, p.CorrelationID)
		if err := h.insightEnqueuer.EnqueueInsightGeneration(ctx, trigger.KgID, recordID, sourceRevision, correlationID, trigger.GeneratorKeys); err != nil {
			return fmt.Errorf("tenant match insights: enqueue: %w", err)
		}
		log.Info("orchestrator: insight generation enqueued", "kg_id", trigger.KgID, "record_id", recordID, "source_revision", sourceRevision)
	}
	return nil
}

func (h *FullPipelineHandler) persistGlobalRecordEnrichment(ctx context.Context, log *slog.Logger, payload []byte) error {
	var p GlobalRecordPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("persist global record enrichment: unmarshal: %w", err)
	}
	persisted, err := h.repo.SaveEnrichment(ctx, &db.RecordEnrichment{
		RecordID:              p.RecordID,
		SourceRevision:        p.SourceRevision,
		Summary:               p.Summary,
		Entities:              p.Entities,
		Topics:                p.Topics,
		KeyClaims:             p.KeyClaims,
		LowConfidenceEntities: p.LowConfidenceEntities,
		Status:                "ready",
	})
	if err != nil {
		return err
	}
	if !persisted {
		log.Info("orchestrator: skipped stale global record enrichment", "record_id", p.RecordID, "source_revision", p.SourceRevision)
		return nil
	}
	matchPayload, err := json.Marshal(TenantMatchPayload{
		RecordID:       p.RecordID,
		CorrelationID:  p.CorrelationID,
		SourceRevision: p.SourceRevision,
	})
	if err != nil {
		return fmt.Errorf("persist global record enrichment: marshal tenant match payload: %w", err)
	}
	if err := h.enqueue(ctx, TaskTenantMatch, p.RecordID, matchPayload); err != nil {
		return err
	}
	// Fan out entity resolution in parallel with tenant matching (both consume the
	// enriched record; resolution feeds the entity layer / graph + vector lenses).
	resolvePayload, err := json.Marshal(ResolveEntitiesPayload{
		RecordID:       p.RecordID,
		CorrelationID:  p.CorrelationID,
		SourceRevision: p.SourceRevision,
	})
	if err != nil {
		return fmt.Errorf("persist global record enrichment: marshal resolve entities payload: %w", err)
	}
	if err := h.enqueue(ctx, TaskResolveEntities, p.RecordID, resolvePayload); err != nil {
		return err
	}
	// Low-confidence search is NOT fanned out here — it's sequenced inside the entity chain
	// (after resolve_entities) so the record advances on a complete entity set.
	log.Info("orchestrator: global record enrichment persisted", "record_id", p.RecordID, "source_revision", p.SourceRevision)
	return nil
}

func (h *FullPipelineHandler) enqueue(ctx context.Context, taskType string, recordID string, payload []byte) error {
	return h.enqueuer.EnqueueStage(ctx, taskType, recordID, payload)
}

func (h *FullPipelineHandler) handleError(ctx context.Context, log *slog.Logger, result Result) error {
	var rateLimitErr ErrRateLimit
	var permanentErr ErrPermanent

	switch {
	case errors.As(result.Err, &rateLimitErr):
		// Return the error so asynq retries the same task.
		// RetryDelayFunc in the server config reads ErrRateLimit.RetryAfter
		// to schedule the retry at the right time.
		log.Warn("orchestrator: rate limited, returning error for asynq retry",
			"retry_after", rateLimitErr.RetryAfter,
		)
		return result.Err

	case errors.As(result.Err, &permanentErr):
		// Mark the record failed so the failure is visible + re-drivable instead of
		// silently dropped. Best-effort — never turn a permanent error into a retry.
		log.Error("orchestrator: permanent error, marking record failed", "err", result.Err)
		if result.RecordID != "" {
			if markErr := h.repo.MarkFailed(ctx, result.RecordID, result.Err.Error()); markErr != nil {
				log.Error("orchestrator: mark failed errored", "err", markErr)
			}
		}
		return nil

	default:
		// Transient or unknown — return error so asynq retries with backoff.
		log.Warn("orchestrator: transient error", "err", result.Err)
		return result.Err
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
