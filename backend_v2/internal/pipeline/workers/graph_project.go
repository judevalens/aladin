package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/pipeline"
)

// GraphSource reads a record's slice of the entity graph from the system of record.
type GraphSource interface {
	RecordGraph(ctx context.Context, recordID string) (graph.GraphData, error)
}

// GraphProjector writes an entity slice into Neo4j (idempotent MERGE).
type GraphProjector interface {
	Project(ctx context.Context, d graph.GraphData) error
}

// GraphProjectWorker projects a completed record's entities into Neo4j —
// the connection lens. Terminal. Only registered when Neo4j is configured, so it never
// runs (and the stage is never enqueued) without a projector.
type GraphProjectWorker struct {
	source    GraphSource
	projector GraphProjector
}

func NewGraphProjectWorker(source GraphSource, projector GraphProjector) *GraphProjectWorker {
	return &GraphProjectWorker{source: source, projector: projector}
}

func (w *GraphProjectWorker) TaskType() string { return pipeline.TaskGraphProject }
func (w *GraphProjectWorker) Concurrency() int { return 3 }

func (w *GraphProjectWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ResolveEntitiesPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskGraphProject,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("graph: unmarshal payload: %w", err)},
		}
	}
	done := pipeline.Result{
		Type:          pipeline.ResultGraphProjectDone,
		TaskType:      pipeline.TaskGraphProject,
		Payload:       raw,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
	data, err := w.source.RecordGraph(ctx, p.RecordID)
	if err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskGraphProject,
			Err:           pipeline.ErrTransient{Cause: fmt.Errorf("graph: read record slice: %w", err)},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}
	if err := w.projector.Project(ctx, data); err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskGraphProject,
			Err:           pipeline.ErrTransient{Cause: fmt.Errorf("graph: project: %w", err)},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}
	return done
}
