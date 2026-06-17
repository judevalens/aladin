package entities

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestResolver_AgainstPostgres exercises the full R0 slice against the sandbox DB:
// the 00010 migration must apply, the repo must round-trip, and the resolver must
// collapse surface variants of one entity while keeping distinct names distinct.
func TestResolver_AgainstPostgres(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test db (00010 entity layer must apply): %v", err)
	}

	recordID := "entity-res-test-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision)
		VALUES ($1, 'story', 'test', 'content', 'enriched', 1)
		ON CONFLICT (id) DO NOTHING
	`, recordID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		// entity_mentions cascade on the record delete; the shared entities created
		// here are global, so clean them by normalized key to keep the sandbox tidy.
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ('openai','anthropic')`)
	})

	r := NewResolver(db.NewEntityRepository(pool))

	ids := map[string]bool{}
	for _, surface := range []string{"OpenAI", "OpenAI Inc", "OpenAI, Inc.", "openai"} {
		id, err := r.Resolve(ctx, Mention{Surface: surface, RecordID: recordID, SourceRevision: 1})
		if err != nil {
			t.Fatalf("resolve %q: %v", surface, err)
		}
		if id == "" {
			t.Fatalf("resolve %q returned empty id", surface)
		}
		ids[id] = true
	}
	if len(ids) != 1 {
		t.Fatalf("expected OpenAI variants to collapse to 1 entity, got %d", len(ids))
	}

	other, err := r.Resolve(ctx, Mention{Surface: "Anthropic", RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve Anthropic: %v", err)
	}
	if ids[other] {
		t.Fatalf("distinct name resolved into the same entity")
	}

	// Re-resolving an already-seen surface must not duplicate the mention.
	if _, err := r.Resolve(ctx, Mention{Surface: "OpenAI", RecordID: recordID, SourceRevision: 1}); err != nil {
		t.Fatalf("re-resolve OpenAI: %v", err)
	}
	var mentions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entity_mentions WHERE record_id = $1`, recordID).Scan(&mentions); err != nil {
		t.Fatalf("count mentions: %v", err)
	}
	// 4 distinct OpenAI surfaces + 1 Anthropic = 5; the re-resolve is a no-op.
	if mentions != 5 {
		t.Fatalf("expected 5 mentions (4 OpenAI surfaces + Anthropic, re-resolve idempotent), got %d", mentions)
	}
}
