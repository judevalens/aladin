package repo

import (
	"context"
	"testing"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// TestArtifactRef_SearchAndReconcile covers the Y2 `#` cross-reference path: unified search
// across claims + artifacts (pages/shards), then the reconcile of a page's projected refs
// (set replaces prior, labels resolved on list).
func TestArtifactRef_SearchAndReconcile(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	pageID := "ref-page-" + uuid.NewString()   // the page holding the refs
	targetPage := "ref-tgt-" + uuid.NewString() // a page it references
	targetShard := "ref-shd-" + uuid.NewString()
	var claimID string

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
	// A tenant thesis claim to reference.
	if err := pool.QueryRow(ctx, `
		INSERT INTO claims (scope, owner_user_id, canonical_text, polarity, trust_tier)
		VALUES ('tenant', $1::uuid, 'Reftarget thesis '||$2||' will win', 'assert', 'verified')
		RETURNING id::text
	`, userID, tag).Scan(&claimID); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM artifact_refs WHERE artifact_id = $1`, pageID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id IN ($1,$2,$3)`, pageID, targetPage, targetShard)
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id = $1::uuid`, claimID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	refRepo := NewArtifactRefPostgres(pool)

	// Unified search: the claim surfaces as kind='claim' (with polarity), the page/shard as
	// kind 'page'/'shard'.
	claimHits, err := refRepo.SearchClaims(ctx, userID, "Reftarget thesis "+tag, 8)
	if err != nil {
		t.Fatalf("search claims: %v", err)
	}
	if len(claimHits) != 1 || claimHits[0].ID != claimID || claimHits[0].Kind != coreservice.RefKindClaim || claimHits[0].Detail != "assert" {
		t.Fatalf("claim hits = %+v", claimHits)
	}
	artHits, err := refRepo.SearchArtifacts(ctx, userID, "Reftarget", 8)
	if err != nil {
		t.Fatalf("search artifacts: %v", err)
	}
	gotKind := map[string]string{}
	for _, h := range artHits {
		gotKind[h.ID] = h.Kind
	}
	if gotKind[targetPage] != coreservice.RefKindPage || gotKind[targetShard] != coreservice.RefKindShard {
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

	// First projection: reference the claim + the target page.
	if err := refRepo.ReplaceRefs(ctx, pageID, []coreservice.ArtifactRef{
		{Kind: coreservice.RefKindClaim, TargetID: claimID, BlockID: "b1", Surface: "thesis"},
		{Kind: coreservice.RefKindPage, TargetID: targetPage, BlockID: "b2", Surface: "note"},
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
	byKind := map[string]coreservice.AttachedRef{}
	for _, r := range refs {
		byKind[r.Kind] = r
	}
	// Labels resolve from the live target, not the stored surface.
	if got := byKind[coreservice.RefKindClaim].Label; got != "Reftarget thesis "+tag+" will win" {
		t.Fatalf("claim ref label = %q", got)
	}
	if got := byKind[coreservice.RefKindPage].Label; got != "Reftarget note "+tag {
		t.Fatalf("page ref label = %q", got)
	}

	// Re-sync with only the shard ref: prior refs drop, the shard is added.
	if err := refRepo.ReplaceRefs(ctx, pageID, []coreservice.ArtifactRef{
		{Kind: coreservice.RefKindShard, TargetID: targetShard, BlockID: "b3", Surface: "shard"},
	}); err != nil {
		t.Fatalf("replace refs 2: %v", err)
	}
	refs, _ = refRepo.ListForArtifact(ctx, pageID)
	if len(refs) != 1 || refs[0].Kind != coreservice.RefKindShard || refs[0].TargetID != targetShard {
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
