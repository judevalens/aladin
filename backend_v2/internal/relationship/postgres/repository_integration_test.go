package postgres

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/relationship"

	"github.com/google/uuid"
)

// TestRelationshipRepo exercises the additive edge layer: upsert idempotency,
// bidirectional lookup, per-user scoping, and delete. Runs against the sandbox DB.
func TestRelationshipRepo(t *testing.T) {
	t.Parallel()
	dsn := dbtest.RequireTestDSN(t)

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	userID := uuid.NewString()
	otherUser := uuid.NewString()
	for _, u := range []string{userID, otherUser} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,created_at) VALUES($1,$2,now())`, u, "rel-"+u+"@test.local"); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		// FK ON DELETE CASCADE removes the user's relationships too.
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = ANY($1)`, []string{userID, otherUser})
	})

	r := NewRelationshipPostgres(pool)
	artifactID := "art-" + uuid.NewString()
	recordID := "rec-" + uuid.NewString()

	// Create an edge: artifact —cites→ record.
	edge := relationship.Relationship{
		UserID: userID, SrcKind: "artifact", SrcID: artifactID,
		DstKind: "record", DstID: recordID, RelType: "cites",
		Metadata: map[string]any{"note": "from the doc"},
	}
	created, err := r.Create(ctx, edge)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("expected id + created_at, got %+v", created)
	}

	// Re-assert the same edge → idempotent (still one row, same id).
	again, err := r.Create(ctx, edge)
	if err != nil {
		t.Fatalf("re-create: %v", err)
	}
	if again.ID != created.ID {
		t.Fatalf("re-assert should keep the same edge id: %s vs %s", again.ID, created.ID)
	}

	// Found from the source side...
	fromSrc, err := r.ListForNode(ctx, userID, "artifact", artifactID)
	if err != nil {
		t.Fatalf("list by src: %v", err)
	}
	if len(fromSrc) != 1 || fromSrc[0].RelType != "cites" || fromSrc[0].Metadata["note"] != "from the doc" {
		t.Fatalf("unexpected edges from source: %+v", fromSrc)
	}
	// ...and from the target side (bidirectional).
	fromDst, err := r.ListForNode(ctx, userID, "record", recordID)
	if err != nil {
		t.Fatalf("list by dst: %v", err)
	}
	if len(fromDst) != 1 || fromDst[0].ID != created.ID {
		t.Fatalf("edge not found from target side: %+v", fromDst)
	}

	// Scoping: another user sees nothing.
	if other, err := r.ListForNode(ctx, otherUser, "artifact", artifactID); err != nil || len(other) != 0 {
		t.Fatalf("scoping leak: other user saw %d edges (err=%v)", len(other), err)
	}

	// Delete → gone.
	if err := r.Delete(ctx, userID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if after, err := r.ListForNode(ctx, userID, "artifact", artifactID); err != nil || len(after) != 0 {
		t.Fatalf("edge not deleted: %d remain (err=%v)", len(after), err)
	}
}
