package entities

import (
	"context"
	"strings"
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

// TestResolver_FuzzyProposesMerge_AndNegativeEvidence: a near-match (one trailing char)
// is NOT auto-merged — it lands as a distinct entity plus a proposed merge — and once a
// human rejects that merge, the pair is never re-proposed (negative evidence).
func TestResolver_FuzzyProposesMerge_AndNegativeEvidence(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	nameA := "novacorp" + tag      // normalized key == itself (lowercase, no suffix)
	nameB := nameA + "x"           // one trailing char → high trigram similarity, distinct key
	recordID := "entity-fuzzy-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision)
		VALUES ($1, 'story', 'test', 'content', 'enriched', 1)
		ON CONFLICT (id) DO NOTHING
	`, recordID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ($1, $2)`, nameA, nameB)
	})

	repo := db.NewEntityRepository(pool)
	r := NewResolver(repo)

	idA, err := r.Resolve(ctx, Mention{Surface: nameA, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve A: %v", err)
	}
	idB, err := r.Resolve(ctx, Mention{Surface: nameB, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve B: %v", err)
	}
	if idA == "" || idB == "" || idA == idB {
		t.Fatalf("fuzzy match must stay distinct (not auto-merged): A=%q B=%q", idA, idB)
	}

	proposals, err := repo.ListProposedMerges(ctx, 100)
	if err != nil {
		t.Fatalf("list proposed: %v", err)
	}
	var mergeID string
	for _, p := range proposals {
		if (p.FromEntityID == idB && p.IntoEntityID == idA) || (p.FromEntityID == idA && p.IntoEntityID == idB) {
			mergeID = p.ID
		}
	}
	if mergeID == "" {
		t.Fatalf("expected a proposed merge for the fuzzy pair %q ~ %q", nameA, nameB)
	}

	if err := repo.RejectMerge(ctx, mergeID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	reproposed, err := repo.ProposeMerge(ctx, db.ProposeMergeParams{
		FromEntityID: idB, IntoEntityID: idA, Confidence: 0.9, Method: "trigram",
	})
	if err != nil {
		t.Fatalf("re-propose: %v", err)
	}
	if reproposed {
		t.Fatal("a rejected pair must not be re-proposed (negative evidence)")
	}
}

// TestEntityRepo_AcceptMergeSetsCanonicalRoot: accepting a proposed merge points the
// from-entity's canonical_root_id at the into-entity (the reversible overlay write).
func TestEntityRepo_AcceptMergeSetsCanonicalRoot(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	keyInto := "acceptinto" + tag
	keyFrom := "acceptfrom" + tag
	repo := db.NewEntityRepository(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ($1, $2)`, keyInto, keyFrom)
	})

	into, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyInto, NormalizedKey: keyInto})
	if err != nil {
		t.Fatalf("create into: %v", err)
	}
	from, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyFrom, NormalizedKey: keyFrom})
	if err != nil {
		t.Fatalf("create from: %v", err)
	}
	if _, err := repo.ProposeMerge(ctx, db.ProposeMergeParams{FromEntityID: from, IntoEntityID: into, Confidence: 0.7, Method: "manual"}); err != nil {
		t.Fatalf("propose: %v", err)
	}

	proposals, err := repo.ListProposedMerges(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var mergeID string
	for _, p := range proposals {
		if p.FromEntityID == from && p.IntoEntityID == into {
			mergeID = p.ID
		}
	}
	if mergeID == "" {
		t.Fatal("expected the proposed merge to be listed")
	}
	if err := repo.AcceptMerge(ctx, mergeID); err != nil {
		t.Fatalf("accept: %v", err)
	}

	var root string
	if err := pool.QueryRow(ctx, `SELECT canonical_root_id::text FROM entities WHERE id = $1::uuid`, from).Scan(&root); err != nil {
		t.Fatalf("query root: %v", err)
	}
	if root != into {
		t.Fatalf("expected from.canonical_root_id = into (%s), got %s", into, root)
	}
}
