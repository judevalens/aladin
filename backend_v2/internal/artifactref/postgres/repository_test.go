package postgres_test

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/artifactref"
	artifactrefpostgres "aladin/backend_v2/internal/artifactref/postgres"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := dbtest.RequireTestDSN(t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func adminContext(userID string) context.Context {
	return coreservice.WithPrincipal(context.Background(), coreservice.Principal{
		UserID: userID, ActorType: coreservice.ActorTypeUserSession, ActorID: userID,
		Scopes: []string{coreservice.ScopeArtifactsRead, coreservice.ScopeArtifactsWrite},
	})
}

// TestArtifactRef_SearchAndReconcile covers the Y2 `#` cross-reference path: search across
// artifacts (pages/shards), then the reconcile of a page's projected refs (set replaces
// prior, labels resolved on list).
func TestArtifactRef_SearchAndReconcile(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	pageID := "ref-page-" + uuid.NewString()    // the page holding the refs
	targetPage := "ref-tgt-" + uuid.NewString() // a page it references
	targetShard := "ref-shd-" + uuid.NewString()
	ctx = adminContext(userID) // ReplaceRefs now emits a node frame → need a principal

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES
			($1, $5::uuid, 'page', 'Home page '||$4, ''),
			($2, $5::uuid, 'page', 'Reftarget note '||$4, ''),
			($3, $5::uuid, 'app',  'Reftarget shard '||$4, '')
	`, pageID, targetPage, targetShard, tag, userID); err != nil {
		t.Fatalf("seed artifacts: %v", err)
	}
	// ReplaceRefs emits a node frame for the holding page → it needs a tree_nodes row.
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, pageID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM artifact_refs WHERE artifact_id = $1`, pageID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id IN ($1,$2,$3)`, pageID, targetPage, targetShard)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	refRepo := artifactrefpostgres.NewArtifactRefPostgres(pool)

	// Search surfaces the page/shard as kind 'page'/'shard'.
	artHits, err := refRepo.SearchArtifacts(ctx, userID, "Reftarget", 8)
	if err != nil {
		t.Fatalf("search artifacts: %v", err)
	}
	gotKind := map[string]string{}
	for _, h := range artHits {
		gotKind[h.ID] = h.Kind
	}
	if gotKind[targetPage] != artifactref.RefKindPage || gotKind[targetShard] != artifactref.RefKindShard {
		t.Fatalf("artifact hits kinds = %+v (%+v)", gotKind, artHits)
	}

	// Scope: another user's search must not see this user's artifacts.
	otherHits, err := refRepo.SearchArtifacts(ctx, uuid.NewString(), "Reftarget", 8)
	if err != nil {
		t.Fatalf("search artifacts (other): %v", err)
	}
	if len(otherHits) != 0 {
		t.Fatalf("expected no cross-tenant artifact hits, got %+v", otherHits)
	}

	// First projection: reference the target page + shard.
	if err := refRepo.ReplaceRefs(ctx, pageID, []artifactref.ArtifactRef{
		{Kind: artifactref.RefKindPage, TargetID: targetPage, BlockID: "b2", Surface: "note"},
		{Kind: artifactref.RefKindShard, TargetID: targetShard, BlockID: "b1", Surface: "shard"},
	}); err != nil {
		t.Fatalf("replace refs: %v", err)
	}
	refs, err := refRepo.ListForArtifact(ctx, pageID)
	if err != nil {
		t.Fatalf("list refs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %+v", refs)
	}
	byKind := map[string]artifactref.AttachedRef{}
	for _, r := range refs {
		byKind[r.Kind] = r
	}
	// Labels resolve from the live target, not the stored surface.
	if got := byKind[artifactref.RefKindPage].Label; got != "Reftarget note "+tag {
		t.Fatalf("page ref label = %q", got)
	}
	if got := byKind[artifactref.RefKindShard].Label; got != "Reftarget shard "+tag {
		t.Fatalf("shard ref label = %q", got)
	}

	// Re-sync with only the shard ref: prior refs drop, the shard is added.
	if err := refRepo.ReplaceRefs(ctx, pageID, []artifactref.ArtifactRef{
		{Kind: artifactref.RefKindShard, TargetID: targetShard, BlockID: "b3", Surface: "shard"},
	}); err != nil {
		t.Fatalf("replace refs 2: %v", err)
	}
	refs, _ = refRepo.ListForArtifact(ctx, pageID)
	if len(refs) != 1 || refs[0].Kind != artifactref.RefKindShard || refs[0].TargetID != targetShard {
		t.Fatalf("after re-sync refs = %+v", refs)
	}

	// Empty sync clears all refs.
	if err := refRepo.ReplaceRefs(ctx, pageID, nil); err != nil {
		t.Fatalf("replace refs empty: %v", err)
	}
	refs, _ = refRepo.ListForArtifact(ctx, pageID)
	if len(refs) != 0 {
		t.Fatalf("expected no refs after empty sync, got %+v", refs)
	}
}
