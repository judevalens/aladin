package repo

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"

	"github.com/google/uuid"
)

// TestGraphPane_ForThesis assembles the pane from a thesis claim: its entity (with
// mention count), a related grounded claim (backed by 2 records), and the cited sources.
func TestGraphPane_ForThesis(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	entName := "novacorp" + tag
	r1 := "gp-r1-" + uuid.NewString()
	r2 := "gp-r2-" + uuid.NewString()

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

	thesisID, err := claimRepo.CreateClaim(ctx, db.CreateClaimParams{Scope: "shared", CanonicalText: entName + " will win", Polarity: "assert", TrustTier: "verified"})
	if err != nil {
		t.Fatalf("create thesis: %v", err)
	}
	if err := claimRepo.AddClaimSubject(ctx, thesisID, eid); err != nil {
		t.Fatalf("thesis subject: %v", err)
	}
	relatedID, err := claimRepo.CreateClaim(ctx, db.CreateClaimParams{Scope: "shared", CanonicalText: entName + " is burning cash", Polarity: "assert"})
	if err != nil {
		t.Fatalf("create related: %v", err)
	}
	if err := claimRepo.AddClaimSubject(ctx, relatedID, eid); err != nil {
		t.Fatalf("related subject: %v", err)
	}
	for _, rid := range []string{r1, r2} {
		if err := claimRepo.AddClaimMention(ctx, db.ClaimMentionParams{ClaimID: relatedID, SourceKind: "record", SourceID: rid, Stance: "assert", Resolver: "test"}); err != nil {
			t.Fatalf("claim mention: %v", err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id IN ($1::uuid, $2::uuid)`, thesisID, relatedID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1::uuid`, eid)
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE id IN ($1, $2)`, r1, r2)
	})

	pane, err := NewGraphPanePostgres(pool).ForThesis(ctx, thesisID)
	if err != nil {
		t.Fatalf("ForThesis: %v", err)
	}
	if pane.Thesis == nil || pane.Thesis.ID != thesisID {
		t.Fatalf("thesis = %+v", pane.Thesis)
	}
	if len(pane.Entities) != 1 || pane.Entities[0].ID != eid || pane.Entities[0].Mentions != 2 || pane.Entities[0].Kind != "org" {
		t.Fatalf("entities = %+v", pane.Entities)
	}
	var related *struct{}
	for _, c := range pane.Claims {
		if c.ID == relatedID {
			if !c.Grounded || c.Sources != 2 {
				t.Fatalf("related claim should be grounded by 2 sources, got %+v", c)
			}
			related = &struct{}{}
		}
		if c.ID == thesisID {
			t.Fatal("the thesis itself must not appear in the claims list")
		}
	}
	if related == nil {
		t.Fatalf("expected the related claim in the pane, got %+v", pane.Claims)
	}
	if len(pane.Cites) != 2 {
		t.Fatalf("expected 2 cited records, got %d", len(pane.Cites))
	}
}
