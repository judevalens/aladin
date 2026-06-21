package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/db"
)

// StuckLister surfaces non-terminal records that have made no progress for a while.
type StuckLister interface {
	ListStuck(ctx context.Context, olderThanSecs, limit int) ([]*db.Record, error)
}

// Reaper re-drives records stranded in a non-terminal status — the backstop for the
// non-transactional capture boundary (a record upserted as 'captured' but whose enrichment
// enqueue was lost to a crash) and for any stage that silently stalled. It's safe to run
// repeatedly: stage task ids are deterministic, so re-enqueuing a task that's still queued
// is a no-op, and every stage is idempotent at the data layer.
type Reaper struct {
	records       StuckLister
	enqueuer      Enqueuer
	olderThanSecs int
	limit         int
}

func NewReaper(records StuckLister, enqueuer Enqueuer) *Reaper {
	return &Reaper{records: records, enqueuer: enqueuer, olderThanSecs: 900, limit: 100}
}

// WithThreshold overrides the stuck threshold (seconds) — smaller in tests.
func (r *Reaper) WithThreshold(secs int) *Reaper {
	r.olderThanSecs = secs
	return r
}

// Sweep re-enqueues the next stage for each stuck record. Returns how many it re-drove.
func (r *Reaper) Sweep(ctx context.Context) (int, error) {
	stuck, err := r.records.ListStuck(ctx, r.olderThanSecs, r.limit)
	if err != nil {
		return 0, fmt.Errorf("reaper: list stuck: %w", err)
	}
	n := 0
	for _, rec := range stuck {
		taskType, payload, err := nextStageFor(rec)
		if err != nil {
			return n, fmt.Errorf("reaper: build payload for %s: %w", rec.ID, err)
		}
		if taskType == "" {
			continue
		}
		if err := r.enqueuer.EnqueueStage(ctx, taskType, rec.ID, payload); err != nil {
			return n, fmt.Errorf("reaper: re-enqueue %s: %w", rec.ID, err)
		}
		n++
	}
	return n, nil
}

// nextStageFor maps a stuck record's status to the stage that should re-drive it.
func nextStageFor(rec *db.Record) (string, []byte, error) {
	switch rec.Status {
	case "pending", "captured":
		payload, err := json.Marshal(GlobalRecordPayload{
			RecordID:         rec.ID,
			ProviderStreamID: rec.ProviderStreamID,
			Provider:         rec.Provider,
			ExternalID:       rec.ExternalID,
			SourceRevision:   rec.SourceRevision,
			Type:             rec.Type,
			Title:            rec.Title,
			ContentExcerpt:   rec.ContentExcerpt,
			SourceURL:        rec.SourceURL,
			Metadata:         rec.ProviderMetadata,
		})
		return TaskGlobalFirstPass, payload, err
	case "enriched":
		// Re-drive the entity→claims→embed chain. The entity stage is idempotent, so this
		// re-runs cleanly.
		payload, err := json.Marshal(ResolveEntitiesPayload{
			RecordID:       rec.ID,
			SourceRevision: rec.SourceRevision,
		})
		return TaskResolveEntities, payload, err
	default:
		return "", nil, nil
	}
}
