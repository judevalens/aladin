package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

// GraphPromoter writes an enriched artifact into Neo4j.
type GraphPromoter interface {
	Promote(ctx context.Context, artifact *db.EmbeddedArtifact) error
}

type GraphWorker struct {
	promoter GraphPromoter
}

func NewGraphWorker(promoter GraphPromoter) *GraphWorker {
	return &GraphWorker{promoter: promoter}
}

func (w *GraphWorker) TaskType()    string { return pipeline.TaskGraph }
func (w *GraphWorker) Concurrency() int    { return 5 }

func (w *GraphWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ArtifactPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskGraph,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("unmarshal payload: %w", err)},
		}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "graph",
		"artifact_id", p.ArtifactID,
		"kg_id", p.KgID,
	)
	start := time.Now()

	if w.promoter != nil {
		enrichment := map[string]any{
			"summary":    p.Summary,
			"entities":   p.Entities,
			"topics":     p.Topics,
			"key_claims": p.KeyClaims,
		}
		if p.SearchResolved != nil {
			enrichment["search_context"] = p.SearchResolved
		}

		artifact := &db.EmbeddedArtifact{
			ID:         p.ArtifactID,
			Type:       p.Type,
			Label:      p.Label,
			SourceURL:  p.SourceURL,
			Enrichment: enrichment,
			CreatedAt:  time.Now(),
		}

		if err := w.promoter.Promote(ctx, artifact); err != nil {
			log.Error("graph: promote failed", "err", err)
			return errResult(pipeline.TaskGraph, p, pipeline.ErrTransient{Cause: fmt.Errorf("promote: %w", err)})
		}
	}

	log.Info("graph: complete", "duration_ms", time.Since(start).Milliseconds())

	payload, err := json.Marshal(p)
	if err != nil {
		return errResult(pipeline.TaskGraph, p, pipeline.ErrPermanent{Cause: err})
	}

	return pipeline.Result{
		Type:       pipeline.ResultGraphDone,
		TaskType:   pipeline.TaskGraph,
		Payload:    payload,
		ArtifactID: p.ArtifactID,
		KgID:       p.KgID,
	}
}
