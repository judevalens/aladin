package workers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline/blackboard"
)

const TaskGraph = "pipeline:graph"

type GraphPromoter interface {
	Promote(ctx context.Context, artifact *db.EmbeddedArtifact) error
}

type GraphResult struct{}

type GraphWorker struct {
	board    *blackboard.Blackboard
	promoter GraphPromoter
}

func NewGraphWorker(
	board *blackboard.Blackboard,
	promoter GraphPromoter,
) *GraphWorker {
	return &GraphWorker{
		board:    board,
		promoter: promoter,
	}
}

func (w *GraphWorker) Register(mux *asynq.ServeMux, onDone func(context.Context, string, *GraphResult, error) error) {
	mux.HandleFunc(TaskGraph, func(ctx context.Context, t *asynq.Task) error {
		id := parseArtifactID(t.Payload())
		result, err := w.Run(ctx, id)
		return onDone(ctx, id, result, err)
	})
}

func (w *GraphWorker) Run(ctx context.Context, artifactID string) (*GraphResult, error) {
	entry, err := w.board.Get(ctx, artifactID)
	if err != nil {
		return nil, ErrTransient{Cause: err}
	}
	if entry == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("entry not found: %s", artifactID)}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "graph",
		"artifact_id", entry.ArtifactID,
		"kg_id", entry.KgID,
	)
	start := time.Now()

	if entry.Raw == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("missing raw artifact")}
	}

	enrichment := map[string]any{}
	if entry.FirstPass != nil {
		enrichment["summary"] = entry.FirstPass.Summary
		enrichment["entities"] = entry.FirstPass.Entities
		enrichment["topics"] = entry.FirstPass.Topics
		enrichment["key_claims"] = entry.FirstPass.KeyClaims
	}
	if entry.Search != nil {
		enrichment["search_context"] = entry.Search.Resolved
	}

	artifact := &db.EmbeddedArtifact{
		ID:         entry.ArtifactID,
		Type:       entry.Raw.Type,
		Label:      entry.Raw.Label,
		SourceURL:  entry.Raw.SourceURL,
		Enrichment: enrichment,
		CreatedAt:  time.Now(),
	}

	if w.promoter != nil {
		if err := w.promoter.Promote(ctx, artifact); err != nil {
			log.Error("graph: promote failed", "err", err)
			return nil, ErrTransient{Cause: fmt.Errorf("graph promote: %w", err)}
		}
	}

	log.Info("graph: complete",
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &GraphResult{}, nil
}
