package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/search"
)

// FullPipelineHandler implements ResultHandler for the full enrichment pipeline:
// first_pass → (optional search) → embed → graph → persist.
// Adding a new pipeline variant means implementing a new ResultHandler — no other changes.
type FullPipelineHandler struct {
	enqueuer        Enqueuer
	repo            db.RecordRepository
	matches         db.TenantItemMatchRepository
	insightEnqueuer InsightEnqueuer
	insights        chan<- string
}

func NewFullPipelineHandler(
	enqueuer Enqueuer,
	repo db.RecordRepository,
	insights chan<- string,
) *FullPipelineHandler {
	return &FullPipelineHandler{enqueuer: enqueuer, repo: repo, insights: insights}
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
		log.Info("orchestrator: global enrichment complete, persisting")
		return h.persistGlobalRecordEnrichment(ctx, log, result.Payload)

	case ResultTenantMatchDone:
		log.Info("orchestrator: tenant matching complete")
		return h.enqueueInsightTriggers(ctx, log, result.Payload)

	case ResultResolveEntitiesDone:
		// Chain claim extraction after entities so claims can ground on the resolved
		// entity set. The payloads share a shape, so forward it directly.
		log.Info("orchestrator: entity resolution complete, routing to claims")
		return h.enqueue(ctx, TaskResolveClaims, result.RecordID, result.Payload)

	case ResultResolveClaimsDone:
		// Terminal: the worker has populated the claim layer directly.
		log.Info("orchestrator: claim extraction complete")
		return nil

	case ResultFirstPassSearchNeeded:
		log.Info("orchestrator: routing to search")
		return h.enqueue(ctx, TaskSearch, result.RecordID, result.Payload)

	case ResultFirstPassEmbedReady:
		log.Info("orchestrator: skipping search, routing to embed")
		return h.enqueue(ctx, TaskEmbed, result.RecordID, result.Payload)

	case ResultSearchDone:
		log.Info("orchestrator: routing to embed")
		return h.enqueue(ctx, TaskEmbed, result.RecordID, result.Payload)

	case ResultEmbedDone:
		log.Info("orchestrator: routing to graph")
		return h.enqueue(ctx, TaskGraph, result.RecordID, result.Payload)

	case ResultGraphDone:
		log.Info("orchestrator: pipeline complete, persisting")
		return h.persist(ctx, log, result.Payload)

	default:
		return fmt.Errorf("orchestrator: unknown result type %q", result.Type)
	}
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
		log.Error("orchestrator: permanent error, dropping", "err", result.Err)
		return nil

	default:
		// Transient or unknown — return error so asynq retries with backoff.
		log.Warn("orchestrator: transient error", "err", result.Err)
		return result.Err
	}
}

func (h *FullPipelineHandler) persist(ctx context.Context, log *slog.Logger, payload []byte) error {
	var p RecordPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("persist: unmarshal: %w", err)
	}

	enrichment, err := json.Marshal(struct {
		Summary       string                           `json:"summary,omitempty"`
		Entities      []string                         `json:"entities,omitempty"`
		Topics        []string                         `json:"topics,omitempty"`
		KeyClaims     []string                         `json:"key_claims,omitempty"`
		SearchContext map[string][]search.SearchResult `json:"search_context,omitempty"`
	}{
		Summary:       p.Summary,
		Entities:      p.Entities,
		Topics:        p.Topics,
		KeyClaims:     p.KeyClaims,
		SearchContext: p.SearchResolved,
	})
	if err != nil {
		log.Error("orchestrator: marshal enrichment failed", "err", err)
		return fmt.Errorf("persist: marshal enrichment: %w", err)
	}

	a := &db.CompletedRecord{
		ID:             p.RecordID,
		ExternalID:     p.ExternalID,
		SourceRevision: p.SourceRevision,
		Type:           p.Type,
		Label:          p.Label,
		Content:        p.Content,
		SourceURL:      p.SourceURL,
		Metadata:       p.Metadata,
		Enrichment:     enrichment,
		Embedding:      p.Embedding,
	}

	if err := h.repo.SaveComplete(ctx, a); err != nil {
		log.Error("orchestrator: save failed", "err", err)
		return err
	}
	select {
	case h.insights <- p.KgID:
	default:
	}

	log.Info("orchestrator: record persisted",
		"correlation_id", p.CorrelationID,
		"external_id", p.ExternalID,
	)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
