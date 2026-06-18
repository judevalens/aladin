package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/claims"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

// ResolveClaimsWorker extracts contestable, entity-grounded claims from an enriched
// record into the claim layer (C0). It runs after resolve_entities so it can ground each
// claim on the record's resolved entities. Terminal — routes to no next stage.
type ResolveClaimsWorker struct {
	records  db.RecordRepository
	entities db.EntityRepository
	claims   *claims.Service
}

func NewResolveClaimsWorker(records db.RecordRepository, entities db.EntityRepository, svc *claims.Service) *ResolveClaimsWorker {
	return &ResolveClaimsWorker{records: records, entities: entities, claims: svc}
}

func (w *ResolveClaimsWorker) TaskType() string { return pipeline.TaskResolveClaims }
func (w *ResolveClaimsWorker) Concurrency() int { return 5 }

func (w *ResolveClaimsWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ResolveClaimsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskResolveClaims,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("resolve_claims: unmarshal payload: %w", err)},
		}
	}
	done := pipeline.Result{
		Type:          pipeline.ResultResolveClaimsDone,
		TaskType:      pipeline.TaskResolveClaims,
		Payload:       raw,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}

	rec, err := w.records.Get(ctx, p.RecordID)
	if err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskResolveClaims,
			Err:           pipeline.ErrTransient{Cause: fmt.Errorf("resolve_claims: get record: %w", err)},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}
	if p.SourceRevision > 0 && rec.SourceRevision > p.SourceRevision {
		return done // stale revision superseded
	}

	ents, err := w.entities.EntitiesForRecord(ctx, rec.ID)
	if err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskResolveClaims,
			Err:           pipeline.ErrTransient{Cause: fmt.Errorf("resolve_claims: entities for record: %w", err)},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}

	if _, err := w.claims.ExtractFromRecord(ctx, claims.RecordInput{
		RecordID:  rec.ID,
		Summary:   rec.Enrichment.Summary,
		KeyClaims: rec.Enrichment.KeyClaims,
		Entities:  ents,
	}); err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskResolveClaims,
			Err:           pipeline.ErrTransient{Cause: err},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}
	return done
}
