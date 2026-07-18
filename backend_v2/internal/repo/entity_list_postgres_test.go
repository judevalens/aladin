package repo

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/entities"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// TestEntityList_Invariants covers the Entities index read path. Each assertion here is
// an invariant that fails SILENTLY if broken — which is why they're tested:
//   - scope: a tenant's private entities must never appear in another user's index
//   - roots only: merged-away entities are ghosts the judge already resolved
//   - attention counts BOTH merge directions (a proposal is a question about both sides)
//   - empty query lists all (the typeahead returns nothing for "" — useless for browsing)
//   - alias search finds an entity by a synonym
func TestEntityList_Invariants(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	me := uuid.NewString()
	other := uuid.NewString()

	sharedName := "Listshared" + tag
	mineName := "Listmine" + tag
	theirsName := "Listtheirs" + tag
	mergedName := "Listmerged" + tag
	aliasSurface := "Listsyn" + tag

	for _, u := range []string{me, other} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
			u, "u-"+u[:8]+"@test.local"); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	var sharedID, mineID, theirsID, mergedID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier, gist)
		VALUES ('shared', 'concept', $1, $2, 'believed', 'a gist') RETURNING id::text
	`, sharedName, entities.Normalize(sharedName)).Scan(&sharedID); err != nil {
		t.Fatalf("seed shared: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, owner_user_id, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('tenant', $1::uuid, 'org', $2, $3, 'placeholder') RETURNING id::text
	`, me, mineName, entities.Normalize(mineName)).Scan(&mineID); err != nil {
		t.Fatalf("seed mine: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, owner_user_id, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('tenant', $1::uuid, 'org', $2, $3, 'believed') RETURNING id::text
	`, other, theirsName, entities.Normalize(theirsName)).Scan(&theirsID); err != nil {
		t.Fatalf("seed theirs: %v", err)
	}
	// A merged-away entity pointing at the shared root.
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, canonical_root_id, trust_tier)
		VALUES ('shared', 'concept', $1, $2, $3::uuid, 'placeholder') RETURNING id::text
	`, mergedName, entities.Normalize(mergedName), sharedID).Scan(&mergedID); err != nil {
		t.Fatalf("seed merged: %v", err)
	}
	// A synonym on the shared entity.
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_aliases (entity_id, surface, normalized) VALUES ($1::uuid, $2, $3)
	`, sharedID, aliasSurface, entities.Normalize(aliasSurface)); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	// A pending proposal: shared → mine. It's a question about BOTH.
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_merges (from_entity_id, into_entity_id, status, confidence, method)
		VALUES ($1::uuid, $2::uuid, 'proposed', 0.78, 'placeholder_sweep')
	`, sharedID, mineID); err != nil {
		t.Fatalf("seed merge: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM entity_merges WHERE from_entity_id = $1::uuid OR into_entity_id = $1::uuid`, sharedID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid,$2::uuid,$3::uuid,$4::uuid)`,
			mergedID, sharedID, mineID, theirsID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id IN ($1::uuid, $2::uuid)`, me, other)
	})

	r := NewEntityListPostgres(pool)
	svc := coreservice.NewEntityListService(r)

	byID := func(items []coreservice.EntityListItem) map[string]coreservice.EntityListItem {
		m := map[string]coreservice.EntityListItem{}
		for _, it := range items {
			m[it.ID] = it
		}
		return m
	}

	// 1. Empty query lists all of MY visible registry — and nobody else's.
	out, err := svc.List(ctx, coreservice.EntityListQuery{OwnerUserID: me})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := byID(out.Entities)
	if _, ok := got[sharedID]; !ok {
		t.Fatal("empty query must list shared entities (browsing is the point)")
	}
	if _, ok := got[mineID]; !ok {
		t.Fatal("my tenant entity must appear in my index")
	}
	if _, ok := got[theirsID]; ok {
		t.Fatal("SCOPE LEAK: another user's tenant entity appeared in my index")
	}
	if _, ok := got[mergedID]; ok {
		t.Fatal("merged-away entity must never surface — the judge already resolved it")
	}

	// 2. Attention counts the proposal from BOTH sides.
	if got[sharedID].Attention != 1 {
		t.Fatalf("expected attention 1 on the from-side, got %d", got[sharedID].Attention)
	}
	if got[mineID].Attention != 1 {
		t.Fatalf("expected attention 1 on the into-side (both directions), got %d", got[mineID].Attention)
	}

	// 3. Aliases ride along, canonical name excluded.
	if len(got[sharedID].Aliases) != 1 || got[sharedID].Aliases[0] != aliasSurface {
		t.Fatalf("expected the synonym on the card, got %v", got[sharedID].Aliases)
	}

	// 4. Searching a SYNONYM finds the entity (alias-aware, like the picker).
	out, err = svc.List(ctx, coreservice.EntityListQuery{OwnerUserID: me, Query: aliasSurface})
	if err != nil {
		t.Fatalf("alias query: %v", err)
	}
	if _, ok := byID(out.Entities)[sharedID]; !ok {
		t.Fatalf("alias query must find the entity, got %+v", out.Entities)
	}

	// 5. Kind filter.
	out, err = svc.List(ctx, coreservice.EntityListQuery{OwnerUserID: me, Kind: "org"})
	if err != nil {
		t.Fatalf("kind filter: %v", err)
	}
	for _, it := range out.Entities {
		if it.Kind != "org" {
			t.Fatalf("kind filter leaked %q", it.Kind)
		}
	}

	// 6. Summary describes the registry and counts the open decision.
	if out.Summary.Total < 2 {
		t.Fatalf("summary total looks wrong: %+v", out.Summary)
	}
	if out.Summary.PendingDecisions < 1 {
		t.Fatalf("expected the pending proposal counted, got %+v", out.Summary)
	}
	if out.Summary.Tiers["placeholder"] < 1 {
		t.Fatalf("expected a placeholder in the tier split, got %+v", out.Summary.Tiers)
	}

	// 7. Status filters (the header pills). "pending" → only entities with an open merge;
	// shared (attention 1) and mine (into-side, attention 1) both qualify; theirs doesn't.
	out, err = svc.List(ctx, coreservice.EntityListQuery{OwnerUserID: me, Filter: coreservice.EntityFilterPending})
	if err != nil {
		t.Fatalf("pending filter: %v", err)
	}
	pending := byID(out.Entities)
	if _, ok := pending[sharedID]; !ok {
		t.Fatal("pending filter must include an entity with an open proposal")
	}
	for _, it := range out.Entities {
		if it.Attention == 0 {
			t.Fatalf("pending filter leaked a no-attention entity: %+v", it)
		}
	}

	// "unresolved" → only placeholder-tier entities (mine is placeholder; shared is believed).
	out, err = svc.List(ctx, coreservice.EntityListQuery{OwnerUserID: me, Filter: coreservice.EntityFilterUnresolved})
	if err != nil {
		t.Fatalf("unresolved filter: %v", err)
	}
	unresolved := byID(out.Entities)
	if _, ok := unresolved[mineID]; !ok {
		t.Fatal("unresolved filter must include the placeholder entity")
	}
	if _, ok := unresolved[sharedID]; ok {
		t.Fatal("unresolved filter must exclude a believed-tier entity")
	}
}
