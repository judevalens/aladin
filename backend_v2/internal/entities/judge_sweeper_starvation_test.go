package entities

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestJudgeSweep_StuckProposalsDoNotStarveNewPlaceholders is the regression guard for the sweeper's
// head-of-line starvation bug.
//
// The queue (ListPlaceholders) is oldest-first and batch-limited. A placeholder whose proposed merge
// can't be decided — similarity in the judge band with no adjudicator configured, so decidePair
// returns "" and the row stays 'proposed' — is never promoted and never leaves the queue head. Past
// `limit` such rows, EVERY sweep re-processed the same stuck set and no newer placeholder was ever
// reached: entity resolution silently stopped (acted==0, and the worker only logs when acted>0).
//
// Here: seed more stuck placeholders than the batch limit, all older than a fresh candidate-less
// placeholder. The fresh one must still be reached and promoted.
func TestJudgeSweep_StuckProposalsDoNotStarveNewPlaceholders(t *testing.T) {
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
	const limit = 5
	const stuckCount = limit + 3 // more stuck rows than a single batch can hold

	var seeded []string
	t.Cleanup(func() {
		bg := context.Background()
		for _, id := range seeded {
			_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1::uuid`, id) // merges cascade
		}
	})

	// newEntity inserts one shared entity, backdated so it sorts ahead of the fresh placeholder.
	newEntity := func(name, tier string, ageMinutes int) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier, created_at)
			VALUES ('shared', 'other', $1, $2, $3, now() - make_interval(mins => $4))
			RETURNING id::text
		`, name, Normalize(name), tier, ageMinutes).Scan(&id); err != nil {
			t.Fatalf("seed entity %s: %v", name, err)
		}
		seeded = append(seeded, id)
		return id
	}

	// The stuck set: each placeholder carries an undecidable 'proposed' merge, so the pre-fix
	// sweeper would re-list it forever.
	for i := 0; i < stuckCount; i++ {
		ph := newEntity("stuckph"+tag+string(rune('a'+i)), "placeholder", 120)
		target := newEntity("stucktarget"+tag+string(rune('a'+i)), "believed", 120)
		if _, err := pool.Exec(ctx, `
			INSERT INTO entity_merges (from_entity_id, into_entity_id, status, confidence, method)
			VALUES ($1::uuid, $2::uuid, 'proposed', 0.60, 'test_stuck')
		`, ph, target); err != nil {
			t.Fatalf("seed stuck proposal: %v", err)
		}
	}

	// The fresh, candidate-less placeholder — newest, so it sits BEHIND the stuck set in the queue.
	// Its name deliberately shares NO substring with the stuck ones (a common tag would make them
	// trigram-similar above fuzzyProposeMinSim, so it would get proposals and legitimately stay a
	// placeholder — testing the wrong thing).
	lone := newEntity("Qwx"+uuid.NewString()[:8], "placeholder", 0)

	// Nil judge: nothing in the band can be decided, exactly the starving configuration.
	if _, err := NewJudgeSweeper(db.NewEntityRepository(pool), nil).Sweep(ctx, limit); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var tier string
	if err := pool.QueryRow(ctx, `SELECT trust_tier FROM entities WHERE id = $1::uuid`, lone).Scan(&tier); err != nil {
		t.Fatalf("read lone tier: %v", err)
	}
	if tier != "believed" {
		t.Fatalf("fresh placeholder tier = %q, want \"believed\": it was starved behind %d stuck "+
			"placeholders (batch limit %d) — the sweeper never reached it", tier, stuckCount, limit)
	}

	// And the stuck rows are untouched — excluding them from the batch must not silently resolve
	// them; they still await a judge.
	var stillProposed int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM entity_merges WHERE method = 'test_stuck' AND status = 'proposed'`).Scan(&stillProposed); err != nil {
		t.Fatalf("count stuck merges: %v", err)
	}
	if stillProposed != stuckCount {
		t.Fatalf("stuck proposals = %d, want %d left pending (they need a judge, not a side effect)",
			stillProposed, stuckCount)
	}
}
