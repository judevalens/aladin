package db

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestClaimRepo_SourceClaims (Y3): the claims a source (artifact) mentions come back via
// SourceClaims — the list the Connect path runs the contradiction surface over.
func TestClaimRepo_SourceClaims(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewClaimRepository(pool)
	owner := uuid.NewString()
	artID := "srcclaim-" + uuid.NewString()

	// Two tenant claims mentioned by this artifact, plus one unrelated claim it doesn't touch.
	c1, err := repo.CreateClaim(ctx, CreateClaimParams{Scope: "tenant", OwnerUserID: owner, CanonicalText: "SourceClaims thesis one", Polarity: "assert", TrustTier: "verified"})
	if err != nil {
		t.Fatalf("create c1: %v", err)
	}
	c2, err := repo.CreateClaim(ctx, CreateClaimParams{Scope: "tenant", OwnerUserID: owner, CanonicalText: "SourceClaims thesis two", Polarity: "deny", TrustTier: "verified"})
	if err != nil {
		t.Fatalf("create c2: %v", err)
	}
	other, err := repo.CreateClaim(ctx, CreateClaimParams{Scope: "tenant", OwnerUserID: owner, CanonicalText: "SourceClaims unrelated", Polarity: "assert"})
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM claim_mentions WHERE source_id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id IN ($1::uuid,$2::uuid,$3::uuid)`, c1, c2, other)
	})

	for _, cid := range []string{c1, c2} {
		if err := repo.AddClaimMention(ctx, ClaimMentionParams{ClaimID: cid, SourceKind: "artifact", SourceID: artID, Stance: "assert", Resolver: "test"}); err != nil {
			t.Fatalf("add mention %s: %v", cid, err)
		}
	}

	got, err := repo.SourceClaims(ctx, "artifact", artID)
	if err != nil {
		t.Fatalf("SourceClaims: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 source claims, got %d (%+v)", len(got), got)
	}
	ids := map[string]string{}
	for _, sc := range got {
		ids[sc.ID] = sc.Polarity
	}
	if ids[c1] != "assert" || ids[c2] != "deny" {
		t.Fatalf("source claims polarity mismatch: %+v", got)
	}
	if _, leaked := ids[other]; leaked {
		t.Fatalf("unrelated claim leaked into SourceClaims: %+v", got)
	}
}
