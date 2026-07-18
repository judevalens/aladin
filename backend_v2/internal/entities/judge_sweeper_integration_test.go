package entities

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestJudgeSweep_AgainstPostgres exercises the P1.3 slice end-to-end on the real repo:
// an editor-minted placeholder whose surface exactly matches an existing entity's ALIAS
// is proposed and deterministically auto-merged (root repointed, one-hop readable);
// a placeholder with no candidates is promoted in place. Nil judge — deterministic tier
// only, so no API keys are needed.
func TestJudgeSweep_AgainstPostgres(t *testing.T) {
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
	realName := "Nvidia" + tag
	realAlias := "nvda" + tag
	phName := "nvda" + tag  // placeholder surface == the real entity's alias
	loneName := "Lonely" + tag

	var realID, phID, loneID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('shared', 'org', $1, $2, 'believed') RETURNING id::text
	`, realName, Normalize(realName)).Scan(&realID); err != nil {
		t.Fatalf("seed real entity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_aliases (entity_id, surface, normalized) VALUES ($1::uuid, $2, $3)
	`, realID, realAlias, Normalize(realAlias)); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('shared', 'other', $1, $2, 'placeholder') RETURNING id::text
	`, phName, Normalize(phName)).Scan(&phID); err != nil {
		t.Fatalf("seed placeholder: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('shared', 'other', $1, $2, 'placeholder') RETURNING id::text
	`, loneName, Normalize(loneName)).Scan(&loneID); err != nil {
		t.Fatalf("seed lone placeholder: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid, $2::uuid, $3::uuid)`, phID, loneID, realID)
	})

	repo := db.NewEntityRepository(pool)
	if _, err := NewJudgeSweeper(repo, nil).Sweep(ctx, 50); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The alias-matched placeholder merged into the real entity: root repointed…
	root, err := repo.ResolveCanonicalRoot(ctx, phID)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	if root != realID {
		t.Fatalf("expected placeholder root %s, got %s", realID, root)
	}
	// …the merge row is applied with the deterministic decider…
	var status, decidedBy string
	if err := pool.QueryRow(ctx, `
		SELECT status, decided_by FROM entity_merges
		 WHERE from_entity_id = $1::uuid AND into_entity_id = $2::uuid
	`, phID, realID).Scan(&status, &decidedBy); err != nil {
		t.Fatalf("read merge row: %v", err)
	}
	if status != "applied" || decidedBy != "auto" {
		t.Fatalf("expected applied/auto, got %s/%s", status, decidedBy)
	}
	// …and it was NOT promoted (merged, not a distinct entity).
	var tier string
	if err := pool.QueryRow(ctx, `SELECT trust_tier FROM entities WHERE id = $1::uuid`, phID).Scan(&tier); err != nil {
		t.Fatalf("read placeholder tier: %v", err)
	}
	if tier != "placeholder" {
		t.Fatalf("merged placeholder should keep its tier (superseded, not promoted), got %q", tier)
	}

	// The lone placeholder had no candidates → promoted in place.
	if err := pool.QueryRow(ctx, `SELECT trust_tier FROM entities WHERE id = $1::uuid`, loneID).Scan(&tier); err != nil {
		t.Fatalf("read lone tier: %v", err)
	}
	if tier != "believed" {
		t.Fatalf("expected lone placeholder promoted to believed, got %q", tier)
	}
}
