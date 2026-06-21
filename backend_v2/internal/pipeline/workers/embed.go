package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/ratelimit"
)

// EmbedWorker computes a record's vector embedding and marks the record complete — the
// terminal stage of the record→knowledge chain (runs after entities + claims). It loads
// the record, embeds label+summary+content, writes records.embedding, and sets status to
// 'complete'. Lean style: minimal payload in, writes its layer directly.
type EmbedWorker struct {
	records  db.RecordRepository
	embedder llm.Embedder
	limiter  *ratelimit.Limiter
}

func NewEmbedWorker(records db.RecordRepository, embedder llm.Embedder, limiter *ratelimit.Limiter) *EmbedWorker {
	return &EmbedWorker{records: records, embedder: embedder, limiter: limiter}
}

func (w *EmbedWorker) TaskType() string { return pipeline.TaskEmbed }
func (w *EmbedWorker) Concurrency() int { return 3 }

func (w *EmbedWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ResolveClaimsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskEmbed,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("embed: unmarshal payload: %w", err)},
		}
	}
	done := pipeline.Result{
		Type:          pipeline.ResultEmbedDone,
		TaskType:      pipeline.TaskEmbed,
		Payload:       raw,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
	fail := func(err error) pipeline.Result {
		return pipeline.Result{TaskType: pipeline.TaskEmbed, Err: err, RecordID: p.RecordID, CorrelationID: p.CorrelationID}
	}

	log := slog.With("component", "pipeline", "stage", "embed", "record_id", p.RecordID, "correlation_id", p.CorrelationID)
	start := time.Now()

	rec, err := w.records.Get(ctx, p.RecordID)
	if err != nil {
		return fail(pipeline.ErrTransient{Cause: fmt.Errorf("embed: get record: %w", err)})
	}
	if p.SourceRevision > 0 && rec.SourceRevision > p.SourceRevision {
		return done // a newer revision superseded this record
	}

	if err := w.limiter.Wait(ctx); err != nil {
		return fail(pipeline.ErrTransient{Cause: err})
	}

	text := rec.Title + "\n" + rec.Enrichment.Summary + "\n" + rec.ContentExcerpt
	vector, err := w.embedder.Embed(ctx, text)
	if err != nil {
		return fail(pipeline.ErrTransient{Cause: fmt.Errorf("embed: %w", err)})
	}

	persisted, err := w.records.SaveEmbedding(ctx, rec.ID, rec.SourceRevision, vector)
	if err != nil {
		return fail(pipeline.ErrTransient{Cause: fmt.Errorf("embed: save: %w", err)})
	}
	if !persisted {
		log.Info("embed: skipped stale embedding")
		return done
	}
	log.Info("embed: complete", "vector_dim", len(vector), "duration_ms", time.Since(start).Milliseconds())
	return done
}
