package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPageIngest_DueSnapshotMark covers the Y1 system-scoped page reads: a page idle past the
// window with edits beyond its last ingest is "due"; the snapshot reads its live owner /
// revision / text; marking it ingested removes it from the due set and is monotonic.
func TestPageIngest_DueSnapshotMark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctx, t)
	defer pool.Close()

	userID := uuid.NewString()
	artID := "page-ing-" + uuid.NewString()

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+uuid.NewString()[:8]+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'Ingest test', '')
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// Page idle for an hour, revision 3, never ingested.
	if _, err := pool.Exec(ctx, `
		INSERT INTO page_documents (artifact_id, revision, search_text, updated_at, last_ingested_revision)
		VALUES ($1, 3, 'hello world', now() - interval '1 hour', 0)
	`, artID); err != nil {
		t.Fatalf("seed page_documents: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id = $1`, artID) // cascades page_documents
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	r := NewArtifactsPostgres(pool)

	// Snapshot reads live state, owner-agnostically.
	snap, err := r.GetPageIngestSnapshot(ctx, artID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.OwnerID != userID || snap.Revision != 3 || snap.LastIngested != 0 || snap.Text != "hello world" {
		t.Fatalf("snapshot = %+v", snap)
	}

	// Due with a 10-min idle window.
	if !pageInDue(t, ctx, r, artID) {
		t.Fatalf("expected page to be due before ingest")
	}

	// Mark ingested at the current revision → no longer due.
	if err := r.MarkPageIngested(ctx, artID, 3); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if pageInDue(t, ctx, r, artID) {
		t.Fatalf("expected page NOT due after ingest")
	}

	// Monotonic: a lower-revision mark is a no-op.
	if err := r.MarkPageIngested(ctx, artID, 2); err != nil {
		t.Fatalf("mark lower: %v", err)
	}
	snap, err = r.GetPageIngestSnapshot(ctx, artID)
	if err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}
	if snap.LastIngested != 3 {
		t.Fatalf("last_ingested = %d, want 3 (monotonic)", snap.LastIngested)
	}
}

func pageInDue(t *testing.T, ctx context.Context, r *PostgresArtifactRepository, artID string) bool {
	t.Helper()
	due, err := r.ListPagesDueForIngest(ctx, 10*time.Minute, 500)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	for _, c := range due {
		if c.ArtifactID == artID {
			return true
		}
	}
	return false
}
