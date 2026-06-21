package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"aladin/backend_v2/internal/claims"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

// SignalEmitter pushes claim frames into the durable sync outbox of the users who should see the
// record (its source's subscribers + owner) — true push of the claim data over ws into the
// client's local store (the Signals surface reads from there). Optional — nil means no push.
type SignalEmitter interface {
	EmitForRecord(ctx context.Context, recordID string) error
}

// ResolveClaimsWorker extracts contestable, entity-grounded claims from an enriched
// record into the claim layer (C0). It runs after resolve_entities so it can ground each
// claim on the record's resolved entities. Terminal — routes to no next stage.
type ResolveClaimsWorker struct {
	records  db.RecordRepository
	entities db.EntityRepository
	claims   *claims.Service
	signals  SignalEmitter
}

func NewResolveClaimsWorker(records db.RecordRepository, entities db.EntityRepository, svc *claims.Service) *ResolveClaimsWorker {
	return &ResolveClaimsWorker{records: records, entities: entities, claims: svc}
}

// WithSignalEmitter enables the realtime "signal.changed" nudge to the record owner after claims
// land. Off (nil) → no nudge; the Signals feed still updates on the client's next manual load.
func (w *ResolveClaimsWorker) WithSignalEmitter(s SignalEmitter) *ResolveClaimsWorker {
	w.signals = s
	return w
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

	stored, err := w.claims.ExtractFromRecord(ctx, claims.RecordInput{
		RecordID:  rec.ID,
		Summary:   rec.Enrichment.Summary,
		KeyClaims: rec.Enrichment.KeyClaims,
		Entities:  ents,
	})
	if err != nil {
		return pipeline.Result{
			TaskType:      pipeline.TaskResolveClaims,
			Err:           pipeline.ErrTransient{Cause: err},
			RecordID:      p.RecordID,
			CorrelationID: p.CorrelationID,
		}
	}

	// Push the record's claims into the owner's durable sync outbox (data_event frames → ws →
	// local SQLite). Owner-scoped: public/ownerless records don't sync to a local store. Returns
	// the error so a failed push retries the whole task (the durability backstop) rather than
	// silently dropping the push.
	if stored > 0 && w.signals != nil {
		if err := w.signals.EmitForRecord(ctx, rec.ID); err != nil {
			return pipeline.Result{
				TaskType:      pipeline.TaskResolveClaims,
				Err:           pipeline.ErrTransient{Cause: fmt.Errorf("resolve_claims: emit signal frames: %w", err)},
				RecordID:      p.RecordID,
				CorrelationID: p.CorrelationID,
			}
		}
		slog.Info("orchestrator: signal frames emitted to outbox",
			"component", "pipeline", "stage", "resolve_claims", "checkpoint", "outbox_emitted",
			"correlation_id", p.CorrelationID, "record_id", rec.ID, "claims", stored)
	}
	return done
}
