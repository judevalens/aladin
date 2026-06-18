package db

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestDiscourseRepo_BridgesAndStore (B2): the 00012 migration applies, a shared entity
// mentioned by 2 records surfaces as a bridge with its members, the ledger versions, and
// a discourse map stores against entity_id.
func TestDiscourseRepo_BridgesAndStore(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate (00012 discourse must apply): %v", err)
	}

	tag := uuid.NewString()[:8]
	entName := "acme" + tag
	r1 := "disc-r1-" + uuid.NewString()
	r2 := "disc-r2-" + uuid.NewString()

	entityRepo := NewEntityRepository(pool)
	eid, err := entityRepo.CreateSharedEntity(ctx, CreateEntityParams{Kind: "unknown", CanonicalName: entName, NormalizedKey: entName})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	for _, rid := range []string{r1, r2} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO records (id, type, label, content, status, source_revision, enrichment)
			VALUES ($1, 'story', $2, 'content', 'enriched', 1, '{"summary":"a summary"}'::jsonb)
			ON CONFLICT (id) DO NOTHING
		`, rid, "label "+rid); err != nil {
			t.Fatalf("seed record: %v", err)
		}
		if err := entityRepo.AddMention(ctx, MentionParams{RecordID: rid, EntityID: eid, Surface: entName, Kind: "unknown", Resolver: "test", SourceRevision: 1}); err != nil {
			t.Fatalf("add mention: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id IN ($1, $2)`, r1, r2)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE id = $1::uuid`, eid)
	})

	repo := NewDiscourseRepository(pool)

	bridges, err := repo.CandidateBridges(ctx, 2, 50)
	if err != nil {
		t.Fatalf("candidate bridges: %v", err)
	}
	var found *Bridge
	for i := range bridges {
		if bridges[i].EntityID == eid {
			found = &bridges[i]
		}
	}
	if found == nil || found.Degree != 2 || found.EntityName != entName {
		t.Fatalf("expected the entity as a degree-2 bridge, got %+v", found)
	}

	members, err := repo.BridgeMembers(ctx, eid, 10)
	if err != nil {
		t.Fatalf("bridge members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	if v, err := repo.MarkAnalyzed(ctx, eid, 2); err != nil || v != 1 {
		t.Fatalf("first MarkAnalyzed: v=%d err=%v", v, err)
	}
	v2, err := repo.MarkAnalyzed(ctx, eid, 2)
	if err != nil || v2 != 2 {
		t.Fatalf("second MarkAnalyzed should bump to 2: v=%d err=%v", v2, err)
	}
	ledger, err := repo.GetDiscourseLedger(ctx, eid)
	if err != nil || ledger == nil || ledger.Version != 2 {
		t.Fatalf("ledger: %+v err=%v", ledger, err)
	}

	if err := repo.StoreDiscourse(ctx, &DiscourseInsight{
		EntityID: eid, EntityName: entName, Title: "discourse on " + entName, Body: "...",
		MemberIDs: []string{r1, r2}, Confidence: 0.8, Version: v2,
	}); err != nil {
		t.Fatalf("store discourse: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM insights WHERE type='discourse' AND entity_id=$1::uuid`, eid).Scan(&n); err != nil {
		t.Fatalf("count discourse insight: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 discourse insight for the entity, got %d", n)
	}
}
