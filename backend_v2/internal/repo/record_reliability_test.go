package repo

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"

	"github.com/google/uuid"
)

// TestRecordReliability covers the I1 capture-boundary guarantees: a stranded record is
// listed as stuck, the revision guard drops stale writes, and the failed→retry transition
// works.
func TestRecordReliability(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	id := "rel-" + uuid.NewString()
	// Seed a 'captured' record whose last touch was an hour ago (stranded by a crash).
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision, created_at, updated_at)
		VALUES ($1, 'post', 'l', 'c', 'captured', 1, now() - interval '2 hours', now() - interval '1 hour')
	`, id); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, id)
	})

	repo := db.NewRecordRepository(pool)

	// ListStuck surfaces it (non-terminal, untouched > threshold).
	stuck, err := repo.ListStuck(ctx, 900, 100)
	if err != nil {
		t.Fatalf("ListStuck: %v", err)
	}
	if !containsRecord(stuck, id) {
		t.Fatalf("ListStuck did not surface the stranded record %s", id)
	}

	// Revision guard: a stale-revision enrichment write is dropped.
	persisted, err := repo.SaveEnrichment(ctx, &db.RecordEnrichment{RecordID: id, SourceRevision: 0, Summary: "stale"})
	if err != nil {
		t.Fatalf("SaveEnrichment: %v", err)
	}
	if persisted {
		t.Fatal("stale-revision enrichment must not persist")
	}

	// MarkFailed → terminal, and ListStuck no longer surfaces it.
	if err := repo.MarkFailed(ctx, id, "boom"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	stuck, _ = repo.ListStuck(ctx, 900, 100)
	if containsRecord(stuck, id) {
		t.Fatal("a failed record must not appear as stuck")
	}
	rec, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Status != "failed" {
		t.Fatalf("status = %q, want failed", rec.Status)
	}

	// ResetForRetry → back to captured, re-drivable.
	reset, err := repo.ResetForRetry(ctx, id)
	if err != nil {
		t.Fatalf("ResetForRetry: %v", err)
	}
	if !reset {
		t.Fatal("ResetForRetry should have reset a failed record")
	}
	rec, _ = repo.Get(ctx, id)
	if rec.Status != "captured" {
		t.Fatalf("after retry status = %q, want captured", rec.Status)
	}
	// A second reset is a no-op (no longer failed).
	reset, _ = repo.ResetForRetry(ctx, id)
	if reset {
		t.Fatal("ResetForRetry on a non-failed record must be a no-op")
	}
}

func containsRecord(recs []*db.Record, id string) bool {
	for _, r := range recs {
		if r.ID == id {
			return true
		}
	}
	return false
}
