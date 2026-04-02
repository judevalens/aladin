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
	enqueuer Enqueuer
	repo     db.ArtifactRepository
	insights chan<- string
}

func NewFullPipelineHandler(
	enqueuer Enqueuer,
	repo db.ArtifactRepository,
	insights chan<- string,
) *FullPipelineHandler {
	return &FullPipelineHandler{enqueuer: enqueuer, repo: repo, insights: insights}
}

func (h *FullPipelineHandler) OnDone(ctx context.Context, result Result) error {
	log := slog.With(
		"component", "orchestrator",
		"artifact_id", result.ArtifactID,
		"kg_id", result.KgID,
		"result_type", result.Type,
	)

	if result.Err != nil {
		return h.handleError(ctx, log, result)
	}

	switch result.Type {
	case ResultFirstPassSearchNeeded:
		log.Info("orchestrator: routing to search")
		return h.enqueue(ctx, TaskSearch, result.ArtifactID, result.Payload)

	case ResultFirstPassEmbedReady:
		log.Info("orchestrator: skipping search, routing to embed")
		return h.enqueue(ctx, TaskEmbed, result.ArtifactID, result.Payload)

	case ResultSearchDone:
		log.Info("orchestrator: routing to embed")
		return h.enqueue(ctx, TaskEmbed, result.ArtifactID, result.Payload)

	case ResultEmbedDone:
		log.Info("orchestrator: routing to graph")
		return h.enqueue(ctx, TaskGraph, result.ArtifactID, result.Payload)

	case ResultGraphDone:
		log.Info("orchestrator: pipeline complete, persisting")
		return h.persist(ctx, log, result.Payload)

	default:
		return fmt.Errorf("orchestrator: unknown result type %q", result.Type)
	}
}

func (h *FullPipelineHandler) enqueue(ctx context.Context, taskType string, artifactID string, payload []byte) error {
	return h.enqueuer.EnqueueStage(ctx, taskType, artifactID, payload)
}

func (h *FullPipelineHandler) handleError(ctx context.Context, log *slog.Logger, result Result) error {
	var rateLimitErr ErrRateLimit
	var permanentErr ErrPermanent

	switch {
	case errors.As(result.Err, &rateLimitErr):
		log.Warn("orchestrator: rate limited, re-enqueuing with delay",
			"retry_after", rateLimitErr.RetryAfter,
		)
		return h.enqueuer.EnqueueStageDelayed(ctx, result.TaskType, result.ArtifactID, result.Payload, rateLimitErr.RetryAfter)

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
	var p ArtifactPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("persist: unmarshal: %w", err)
	}

	enrichment, _ := json.Marshal(struct {
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

	a := &db.CompletedArtifact{
		ID:         p.ArtifactID,
		ExternalID: p.ExternalID,
		SourceID:   p.SourceID,
		Type:       p.Type,
		Label:      p.Label,
		Content:    p.Content,
		SourceURL:  p.SourceURL,
		Metadata:   p.Metadata,
		Enrichment: enrichment,
		Embedding:  p.Embedding,
	}

	if err := h.repo.SaveComplete(ctx, a); err != nil {
		log.Error("orchestrator: save failed", "err", err)
		return err
	}

	select {
	case h.insights <- p.KgID:
	default:
	}

	log.Info("orchestrator: artifact persisted",
		"external_id", p.ExternalID,
		"source_id", p.SourceID,
	)
	return nil
}
