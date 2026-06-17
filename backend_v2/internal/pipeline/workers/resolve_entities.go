package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/pipeline"
)

// ResolveEntitiesWorker resolves a record's enrichment entity strings into the
// canonical entity registry (R0). It runs off the enriched record, in parallel with
// tenant matching; it populates the entity layer and routes to no next stage.
type ResolveEntitiesWorker struct {
	records  db.RecordRepository
	resolver *entities.Resolver
}

func NewResolveEntitiesWorker(records db.RecordRepository, resolver *entities.Resolver) *ResolveEntitiesWorker {
	return &ResolveEntitiesWorker{records: records, resolver: resolver}
}

func (w *ResolveEntitiesWorker) TaskType() string { return pipeline.TaskResolveEntities }
func (w *ResolveEntitiesWorker) Concurrency() int { return 5 }

func (w *ResolveEntitiesWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ResolveEntitiesPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskResolveEntities,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("resolve_entities: unmarshal payload: %w", err)},
		}
	}
	done := pipeline.Result{
		Type:          pipeline.ResultResolveEntitiesDone,
		TaskType:      pipeline.TaskResolveEntities,
		Payload:       raw,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}

	rec, err := w.records.Get(ctx, p.RecordID)
	if err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskResolveEntities,
			Err:           pipeline.ErrTransient{Cause: fmt.Errorf("resolve_entities: get record: %w", err)},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}
	// Revision guard: a newer revision is already in flight, so this resolution is
	// stale — drop it rather than write entities from superseded enrichment.
	if p.SourceRevision > 0 && rec.SourceRevision > p.SourceRevision {
		return done
	}

	// R0 resolves the high-confidence entities only; low_confidence_entities are
	// deliberately left out of the registry (precision bias) until a later phase.
	for _, surface := range rec.Enrichment.Entities {
		if _, err := w.resolver.Resolve(ctx, entities.Mention{
			Surface:        surface,
			RecordID:       rec.ID,
			SourceRevision: rec.SourceRevision,
		}); err != nil {
			return pipeline.Result{
				TaskType:      pipeline.TaskResolveEntities,
				Err:           pipeline.ErrTransient{Cause: err},
				RecordID:      p.RecordID,
				CorrelationID: p.CorrelationID,
			}
		}
	}
	return done
}
