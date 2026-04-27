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

type FirstPassWorker struct {
	enricher llm.Enricher
	limiter  *ratelimit.Limiter
}

func NewFirstPassWorker(enricher llm.Enricher, limiter *ratelimit.Limiter) *FirstPassWorker {
	return &FirstPassWorker{enricher: enricher, limiter: limiter}
}

func (w *FirstPassWorker) TaskType() string { return pipeline.TaskFirstPass }
func (w *FirstPassWorker) Concurrency() int { return 10 }

func (w *FirstPassWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.RecordPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskFirstPass,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("unmarshal payload: %w", err)},
		}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "first_pass",
		"record_id", p.RecordID,
		"correlation_id", p.CorrelationID,
		"kg_id", p.KgID,
	)
	start := time.Now()

	if p.Content == "" {
		return errResult(pipeline.TaskFirstPass, p, pipeline.ErrPermanent{Cause: fmt.Errorf("missing content")})
	}

	if err := w.limiter.Wait(ctx); err != nil {
		return errResult(pipeline.TaskFirstPass, p, pipeline.ErrTransient{Cause: err})
	}

	log.Debug("first_pass: calling enricher", "content_len", len(p.Content), "type", p.Type)

	result, err := w.enricher.Enrich(ctx, p.Content, p.Type)
	if err != nil {
		log.Error("first_pass: enrich failed", "err", err)
		return errResult(pipeline.TaskFirstPass, p, pipeline.ErrTransient{Cause: fmt.Errorf("enrich: %w", err)})
	}

	p.Summary = result.Summary
	p.Entities = result.Entities
	p.Topics = result.Topics
	p.KeyClaims = result.KeyClaims
	p.LowConfidenceEntities = result.LowConfidenceEntities

	log.Info("first_pass: complete",
		"entity_count", len(result.Entities),
		"low_confidence_count", len(result.LowConfidenceEntities),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	payload, err := json.Marshal(p)
	if err != nil {
		return errResult(pipeline.TaskFirstPass, p, pipeline.ErrPermanent{Cause: err})
	}

	resultType := pipeline.ResultFirstPassEmbedReady
	if len(p.LowConfidenceEntities) > 0 {
		resultType = pipeline.ResultFirstPassSearchNeeded
	}

	return pipeline.Result{
		Type:          resultType,
		TaskType:      pipeline.TaskFirstPass,
		Payload:       payload,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
		KgID:          p.KgID,
	}
}

func errResult(taskType string, p pipeline.RecordPayload, err error) pipeline.Result {
	return pipeline.Result{
		TaskType:      taskType,
		Err:           err,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
		KgID:          p.KgID,
	}
}
