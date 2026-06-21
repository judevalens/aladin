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
