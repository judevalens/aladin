package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/ratelimit"
)

type EmbedWorker struct {
	embedder llm.Embedder
	limiter  *ratelimit.Limiter
}

func NewEmbedWorker(embedder llm.Embedder, limiter *ratelimit.Limiter) *EmbedWorker {
	return &EmbedWorker{embedder: embedder, limiter: limiter}
}

func (w *EmbedWorker) TaskType()    string { return pipeline.TaskEmbed }
func (w *EmbedWorker) Concurrency() int    { return 3 }

func (w *EmbedWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ArtifactPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskEmbed,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("unmarshal payload: %w", err)},
		}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "embed",
		"artifact_id", p.ArtifactID,
		"correlation_id", p.CorrelationID,
		"kg_id", p.KgID,
	)
	start := time.Now()

	if err := w.limiter.Wait(ctx); err != nil {
		return errResult(pipeline.TaskEmbed, p, pipeline.ErrTransient{Cause: err})
	}

	text := p.Label + "\n" + p.Summary + "\n" + p.Content
	log.Debug("embed: calling embedder", "text_len", len(text))

	vector, err := w.embedder.Embed(ctx, text)
	if err != nil {
		log.Error("embed: failed", "err", err)
		return errResult(pipeline.TaskEmbed, p, pipeline.ErrTransient{Cause: fmt.Errorf("embed: %w", err)})
	}

	p.Embedding = vector

	log.Info("embed: complete",
		"vector_dim", len(vector),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	payload, err := json.Marshal(p)
	if err != nil {
		return errResult(pipeline.TaskEmbed, p, pipeline.ErrPermanent{Cause: err})
	}

	return pipeline.Result{
		Type:       pipeline.ResultEmbedDone,
		TaskType:   pipeline.TaskEmbed,
		Payload:    payload,
		ArtifactID: p.ArtifactID,
		CorrelationID: p.CorrelationID,
		KgID:       p.KgID,
	}
}
