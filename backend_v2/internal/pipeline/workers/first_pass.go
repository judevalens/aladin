package workers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline/blackboard"
	"aladin/backend_v2/internal/ratelimit"
)

const TaskFirstPass = "pipeline:first_pass"

type FirstPassResult struct {
	Summary               string
	Entities              []string
	Topics                []string
	KeyClaims             []string
	LowConfidenceEntities []string
}

type FirstPassWorker struct {
	board    *blackboard.Blackboard
	enricher llm.Enricher
	limiter  *ratelimit.Limiter
}

func NewFirstPassWorker(
	board *blackboard.Blackboard,
	enricher llm.Enricher,
	limiter *ratelimit.Limiter,
) *FirstPassWorker {
	return &FirstPassWorker{board: board, enricher: enricher, limiter: limiter}
}

// Register wires this worker's handler onto the mux. The worker owns its queue;
// when a task completes it calls onDone so the orchestrator can route next stage.
func (w *FirstPassWorker) Register(mux *asynq.ServeMux, onDone func(context.Context, string, *FirstPassResult, error) error) {
	mux.HandleFunc(TaskFirstPass, func(ctx context.Context, t *asynq.Task) error {
		id := parseArtifactID(t.Payload())
		result, err := w.Run(ctx, id)
		return onDone(ctx, id, result, err)
	})
}

func (w *FirstPassWorker) Run(ctx context.Context, artifactID string) (*FirstPassResult, error) {
	entry, err := w.board.Get(ctx, artifactID)
	if err != nil {
		return nil, ErrTransient{Cause: err}
	}
	if entry == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("entry not found: %s", artifactID)}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "first_pass",
		"artifact_id", entry.ArtifactID,
		"kg_id", entry.KgID,
	)
	start := time.Now()

	if entry.Raw == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("missing raw artifact")}
	}

	if err := w.limiter.Wait(ctx); err != nil {
		return nil, ErrTransient{Cause: err}
	}

	log.Debug("first_pass: calling enricher",
		"content_len", len(entry.Raw.Content),
		"type", entry.Raw.Type,
	)

	result, err := w.enricher.Enrich(ctx, entry.Raw.Content, entry.Raw.Type)
	if err != nil {
		log.Error("first_pass: enrich failed", "err", err)
		return nil, ErrTransient{Cause: fmt.Errorf("first pass enrich: %w", err)}
	}

	log.Info("first_pass: complete",
		"entity_count", len(result.Entities),
		"low_confidence_count", len(result.LowConfidenceEntities),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &FirstPassResult{
		Summary:               result.Summary,
		Entities:              result.Entities,
		Topics:                result.Topics,
		KeyClaims:             result.KeyClaims,
		LowConfidenceEntities: result.LowConfidenceEntities,
	}, nil
}
