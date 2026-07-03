package repo

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// TestSignalRepo_List seeds a shared claim with subjects, an assert/deny evidence split, and a
// contradicts edge, then asserts the Signals surface assembles the full card + signal score.
func TestSignalRepo_List(t *testing.T) {
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

	tag := uuid.NewString()[:8]
	entID := uuid.NewString()
	var c1, c2 string

	// entity (subject)
	if _, err := pool.Exec(ctx, `
		INSERT INTO entities (id, scope, kind, canonical_name, normalized_key)
		VALUES ($1, 'shared', 'org', $2, $3)
	`, entID, "Acme "+tag, "acme-"+tag); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	// two shared claims
	if err := pool.QueryRow(ctx, `INSERT INTO claims (scope, canonical_text, polarity) VALUES ('shared',$1,'assert') RETURNING id::text`,
		"Acme will ship "+tag).Scan(&c1); err != nil {
		t.Fatalf("seed claim1: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO claims (scope, canonical_text, polarity) VALUES ('shared',$1,'assert') RETURNING id::text`,
		"Acme will fail "+tag).Scan(&c2); err != nil {
		t.Fatalf("seed claim2: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id = ANY($1::uuid[])`, []string{c1, c2})
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1`, entID)
	})

	// subject link, evidence (2 assert + 1 deny), and a contradicts edge c1 -> c2
	if _, err := pool.Exec(ctx, `INSERT INTO claim_subjects (claim_id, entity_id) VALUES ($1::uuid,$2::uuid)`, c1, entID); err != nil {
		t.Fatalf("seed subject: %v", err)
	}
	for i, st := range []string{"assert", "assert", "deny"} {
		if _, err := pool.Exec(ctx, `INSERT INTO claim_mentions (claim_id, source_kind, source_id, stance) VALUES ($1::uuid,'record',$2,$3)`,
			c1, uuid.NewString(), st); err != nil {
			t.Fatalf("seed mention %d: %v", i, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO claim_edges (from_claim_id, to_claim_id, type, status) VALUES ($1::uuid,$2::uuid,'contradicts','proposed')`,
		c1, c2); err != nil {
		t.Fatalf("seed edge: %v", err)
	}

	out, err := NewSignalPostgres(pool).List(ctx, service.SignalListParams{Limit: 100, Offset: 0, Sort: "recent"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	signals, _ := out["signals"].([]service.ClaimSignal)
	var got *service.ClaimSignal
	for i := range signals {
		if signals[i].ID == c1 {
			got = &signals[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("seeded claim %s not in signals (n=%d)", c1, len(signals))
	}
	if len(got.Subjects) != 1 || got.Subjects[0].Name != "Acme "+tag {
		t.Fatalf("subjects = %+v, want [Acme %s]", got.Subjects, tag)
	}
	if got.AssertCount != 2 || got.DenyCount != 1 {
		t.Fatalf("assert/deny = %d/%d, want 2/1", got.AssertCount, got.DenyCount)
	}
	if got.Contradicts != 1 {
		t.Fatalf("contradicts = %d, want 1", got.Contradicts)
	}
	// score = (2+1) + 2*min(2,1) + (0+1+0) = 6
	if got.SignalScore != 6 {
		t.Fatalf("signalScore = %v, want 6", got.SignalScore)
	}
}

// TestSignalRepo_ListBook seeds an authored thesis plus two discovered (shared) claims of the
// same proposition — one supported by 2 sources, one denied by 1 — and asserts the book lens
// marks the thesis to market (support=2, contradict=1) and returns it only to its owner.
func TestSignalRepo_ListBook(t *testing.T) {
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

	tag := uuid.NewString()[:8]
	entID := uuid.NewString()
	owner := uuid.NewString()
	vec := vec1536(1) // all three claims share one embedding → cosine 1.0, same proposition
	var thesis, d1, d2 string

	if _, err := pool.Exec(ctx, `
		INSERT INTO entities (id, scope, kind, canonical_name, normalized_key)
		VALUES ($1, 'shared', 'org', $2, $3)
	`, entID, "Acme "+tag, "acme-book-"+tag); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	// the authored thesis (tenant-scoped, owned) with an embedding
	if err := pool.QueryRow(ctx, `
		INSERT INTO claims (scope, owner_user_id, canonical_text, polarity, embedding)
		VALUES ('tenant', $1::uuid, $2, 'assert', $3::vector) RETURNING id::text
	`, owner, "Acme will win "+tag, vec).Scan(&thesis); err != nil {
		t.Fatalf("seed thesis: %v", err)
	}
	// two discovered (shared) claims of the same proposition
	if err := pool.QueryRow(ctx, `INSERT INTO claims (scope, canonical_text, polarity, embedding) VALUES ('shared',$1,'assert',$2::vector) RETURNING id::text`,
		"Acme is winning "+tag, vec).Scan(&d1); err != nil {
		t.Fatalf("seed d1: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO claims (scope, canonical_text, polarity, embedding) VALUES ('shared',$1,'deny',$2::vector) RETURNING id::text`,
		"Acme is losing "+tag, vec).Scan(&d2); err != nil {
		t.Fatalf("seed d2: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id = ANY($1::uuid[])`, []string{thesis, d1, d2})
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1`, entID)
	})

	// all three claims share the subject entity
	for _, cid := range []string{thesis, d1, d2} {
		if _, err := pool.Exec(ctx, `INSERT INTO claim_subjects (claim_id, entity_id) VALUES ($1::uuid,$2::uuid)`, cid, entID); err != nil {
			t.Fatalf("seed subject %s: %v", cid, err)
		}
	}
	// 2 distinct sources assert d1 (support), 1 source denies d2 (contradict)
	for _, st := range []struct{ claim, stance string }{{d1, "assert"}, {d1, "assert"}, {d2, "deny"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO claim_mentions (claim_id, source_kind, source_id, stance) VALUES ($1::uuid,'record',$2,$3)`,
			st.claim, uuid.NewString(), st.stance); err != nil {
			t.Fatalf("seed mention: %v", err)
		}
	}

	repo := NewSignalPostgres(pool)
	out, err := repo.List(ctx, service.SignalListParams{Limit: 100, Lens: "book", OwnerUserID: owner})
	if err != nil {
		t.Fatalf("List book: %v", err)
	}
	signals, _ := out["signals"].([]service.ClaimSignal)
	if len(signals) != 1 {
		t.Fatalf("book lens returned %d signals, want 1", len(signals))
	}
	got := signals[0]
	if got.ID != thesis {
		t.Fatalf("book signal id = %s, want thesis %s", got.ID, thesis)
	}
	if got.AssertCount != 2 || got.DenyCount != 1 {
		t.Fatalf("support/contradict = %d/%d, want 2/1", got.AssertCount, got.DenyCount)
	}
	// score = (2+1) + 2*min(2,1) = 5
	if got.SignalScore != 5 {
		t.Fatalf("signalScore = %v, want 5", got.SignalScore)
	}
	if len(got.Subjects) != 1 || got.Subjects[0].Name != "Acme "+tag {
		t.Fatalf("subjects = %+v, want [Acme %s]", got.Subjects, tag)
	}

	// a different user's book is empty (ownership scoping)
	otherOut, err := repo.List(ctx, service.SignalListParams{Limit: 100, Lens: "book", OwnerUserID: uuid.NewString()})
	if err != nil {
		t.Fatalf("List book (other): %v", err)
	}
	if other, _ := otherOut["signals"].([]service.ClaimSignal); len(other) != 0 {
		t.Fatalf("other user's book = %d signals, want 0", len(other))
	}
}
