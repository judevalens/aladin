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

const TaskEmbed = "pipeline:embed"

type EmbedResult struct {
	Embedding []float32
}

type EmbedWorker struct {
	board    *blackboard.Blackboard
	embedder llm.Embedder
	limiter  *ratelimit.Limiter
}

func NewEmbedWorker(
	board *blackboard.Blackboard,
	embedder llm.Embedder,
	limiter *ratelimit.Limiter,
) *EmbedWorker {
	return &EmbedWorker{board: board, embedder: embedder, limiter: limiter}
}

func (w *EmbedWorker) Register(mux *asynq.ServeMux, onDone func(context.Context, string, *EmbedResult, error) error) {
	mux.HandleFunc(TaskEmbed, func(ctx context.Context, t *asynq.Task) error {
		id := parseArtifactID(t.Payload())
		result, err := w.Run(ctx, id)
		return onDone(ctx, id, result, err)
	})
}

func (w *EmbedWorker) Run(ctx context.Context, artifactID string) (*EmbedResult, error) {
	entry, err := w.board.Get(ctx, artifactID)
	if err != nil {
		return nil, ErrTransient{Cause: err}
	}
	if entry == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("entry not found: %s", artifactID)}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "embed",
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

	summary := ""
	if entry.FirstPass != nil {
		summary = entry.FirstPass.Summary
	}
	text := entry.Raw.Label + "\n" + summary + "\n" + entry.Raw.Content

	log.Debug("embed: calling embedder", "text_len", len(text))

	vector, err := w.embedder.Embed(ctx, text)
	if err != nil {
		log.Error("embed: failed", "err", err)
		return nil, ErrTransient{Cause: fmt.Errorf("embed: %w", err)}
	}

	log.Info("embed: complete",
		"vector_dim", len(vector),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &EmbedResult{Embedding: vector}, nil
}
