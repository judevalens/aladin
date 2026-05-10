package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/ratelimit"
)

type GlobalFirstPassWorker struct {
	enricher llm.Enricher
	limiter  *ratelimit.Limiter
}

func NewGlobalFirstPassWorker(enricher llm.Enricher, limiter *ratelimit.Limiter) *GlobalFirstPassWorker {
	return &GlobalFirstPassWorker{enricher: enricher, limiter: limiter}
}

func (w *GlobalFirstPassWorker) TaskType() string { return pipeline.TaskGlobalFirstPass }
func (w *GlobalFirstPassWorker) Concurrency() int { return 10 }

func (w *GlobalFirstPassWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.SourceItemPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskGlobalFirstPass,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("unmarshal source item payload: %w", err)},
		}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "global_first_pass",
		"source_item_id", p.SourceItemID,
		"provider_stream_id", p.ProviderStreamID,
		"correlation_id", p.CorrelationID,
	)
	start := time.Now()

	content := strings.TrimSpace(p.ContextExcerpt)
	if content == "" {
		content = strings.TrimSpace(p.ContentExcerpt)
	}
	if content == "" {
		return sourceItemErrResult(p, pipeline.ErrPermanent{Cause: fmt.Errorf("missing source item content")})
	}
	if err := w.limiter.Wait(ctx); err != nil {
		return sourceItemErrResult(p, pipeline.ErrTransient{Cause: err})
	}

	result, err := w.enricher.Enrich(ctx, content, p.Type)
	if err != nil {
		log.Error("global_first_pass: enrich failed", "err", err)
		return sourceItemErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("enrich: %w", err)})
	}

	p.Summary = result.Summary
	p.Entities = result.Entities
	p.Topics = result.Topics
	p.KeyClaims = result.KeyClaims
	p.LowConfidenceEntities = result.LowConfidenceEntities

	payload, err := json.Marshal(p)
	if err != nil {
		return sourceItemErrResult(p, pipeline.ErrPermanent{Cause: err})
	}

	log.Info("global_first_pass: complete",
		"entity_count", len(result.Entities),
		"topic_count", len(result.Topics),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return pipeline.Result{
		Type:          pipeline.ResultGlobalFirstPassDone,
		TaskType:      pipeline.TaskGlobalFirstPass,
		Payload:       payload,
		RecordID:      p.SourceItemID,
		CorrelationID: p.CorrelationID,
	}
}

func sourceItemErrResult(p pipeline.SourceItemPayload, err error) pipeline.Result {
	return pipeline.Result{
		TaskType:      pipeline.TaskGlobalFirstPass,
		Err:           err,
		RecordID:      p.SourceItemID,
		CorrelationID: p.CorrelationID,
	}
}
