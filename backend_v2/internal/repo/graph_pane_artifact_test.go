package repo

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"

	"github.com/google/uuid"
)

// TestGraphPane_ForArtifact assembles the pane for an artifact you're viewing: a claim
// grounded in it (authored extraction), the entity that claim is about (with mention
// count), and the record sources backing the claim.
func TestGraphPane_ForArtifact(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	entName := "acmecorp" + tag
	artID := "gp-art-" + uuid.NewString()
	r1 := "gp-ar1-" + uuid.NewString()
	r2 := "gp-ar2-" + uuid.NewString()

	entityRepo := db.NewEntityRepository(pool)
	claimRepo := db.NewClaimRepository(pool)

	eid, err := entityRepo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "org", CanonicalName: entName, NormalizedKey: entName})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	for _, rid := range []string{r1, r2} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO records (id, type, label, content, status, source_revision, provider, source_url)
			VALUES ($1, 'story', $2, 'content', 'enriched', 1, 'hackernews', 'https://x/'||$1)
			ON CONFLICT (id) DO NOTHING
		`, rid, "label "+rid); err != nil {
			t.Fatalf("seed record: %v", err)
		}
		if err := entityRepo.AddMention(ctx, db.MentionParams{RecordID: rid, EntityID: eid, Surface: entName, Kind: "org", Resolver: "test", SourceRevision: 1}); err != nil {
			t.Fatalf("entity mention: %v", err)
		}
	}

	claimID, err := claimRepo.CreateClaim(ctx, db.CreateClaimParams{Scope: "shared", CanonicalText: entName + " is burning cash", Polarity: "assert"})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if err := claimRepo.AddClaimSubject(ctx, claimID, eid); err != nil {
		t.Fatalf("claim subject: %v", err)
	}
	// Ground the claim in the artifact you're viewing.
	if err := claimRepo.AddClaimMention(ctx, db.ClaimMentionParams{ClaimID: claimID, SourceKind: "artifact", SourceID: artID, Stance: "assert", Resolver: "test"}); err != nil {
		t.Fatalf("artifact claim mention: %v", err)
	}
	// And in two record sources, so it reads as grounded by 2 sources.
	for _, rid := range []string{r1, r2} {
		if err := claimRepo.AddClaimMention(ctx, db.ClaimMentionParams{ClaimID: claimID, SourceKind: "record", SourceID: rid, Stance: "assert", Resolver: "test"}); err != nil {
			t.Fatalf("record claim mention: %v", err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id = $1::uuid`, claimID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1::uuid`, eid)
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE id IN ($1, $2)`, r1, r2)
	})

	pane, err := NewGraphPanePostgres(pool).ForArtifact(ctx, artID)
	if err != nil {
		t.Fatalf("ForArtifact: %v", err)
	}
	if pane.Thesis != nil {
		t.Fatalf("artifact pane has no single thesis, got %+v", pane.Thesis)
	}
	if len(pane.Claims) != 1 || pane.Claims[0].ID != claimID || !pane.Claims[0].Grounded || pane.Claims[0].Sources != 2 {
		t.Fatalf("claims = %+v", pane.Claims)
	}
	if len(pane.Entities) != 1 || pane.Entities[0].ID != eid || pane.Entities[0].Mentions != 2 || pane.Entities[0].Kind != "org" {
		t.Fatalf("entities = %+v", pane.Entities)
	}
	if len(pane.Cites) != 2 {
		t.Fatalf("expected 2 cited records, got %d", len(pane.Cites))
	}
}
