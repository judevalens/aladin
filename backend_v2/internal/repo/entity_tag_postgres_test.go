package repo

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// TestEntityTag_SearchAttachDetach covers the P1 tag path: create an entity, find it via
// the typeahead, tag a page with it (idempotently), see it surface in the graph pane as a
// 'tag'-origin entity, then detach it.
func TestEntityTag_SearchAttachDetach(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artID := "tag-art-" + uuid.NewString()
	name := "Tagco" + tag
	ctx = adminContext(userID) // attach/detach now emit a node frame → need a principal

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'History test', '')
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// The tag/mention writes now emit a node frame; that needs the artifact's tree_nodes row.
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, artID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id = $1`, artID) // cascades artifact_entities
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE canonical_name = $1`, name)
	})

	tagRepo := NewEntityTagPostgres(pool)

	// Create new entity via the "create new" path.
	hit, err := tagRepo.CreateEntity(ctx, "org", name, entities.Normalize(name))
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if hit.ID == "" || hit.Name != name || hit.Kind != "org" || hit.Scope != "shared" {
		t.Fatalf("create hit = %+v", hit)
	}

	// Typeahead finds it. Search the full unique name: a bare "Tagco" prefix query is
	// nondeterministic against rows leaked by killed test runs (alphabetical tiebreak
	// over random suffixes decides who makes the LIMIT), which made this test flaky.
	hits, err := tagRepo.SearchEntities(ctx, userID, name, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, h := range hits {
		if h.ID == hit.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("search did not return the new entity; got %+v", hits)
	}

	// Tag the page; idempotent on repeat.
	if err := tagRepo.AttachTag(ctx, artID, hit.ID, userID); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := tagRepo.AttachTag(ctx, artID, hit.ID, userID); err != nil {
		t.Fatalf("attach (repeat): %v", err)
	}
	attached, err := tagRepo.ListForArtifact(ctx, artID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(attached) != 1 || attached[0].ID != hit.ID || attached[0].Origin != "tag" {
		t.Fatalf("attached = %+v", attached)
	}

	// The tag is the authored-extraction grounding set (P3 bridge).
	refs, err := db.NewEntityRepository(pool).EntitiesForArtifact(ctx, artID)
	if err != nil {
		t.Fatalf("EntitiesForArtifact: %v", err)
	}
	if len(refs) != 1 || refs[0].ID != hit.ID || refs[0].Name != name {
		t.Fatalf("grounding refs = %+v", refs)
	}

	// It surfaces in the graph pane as a tag-origin entity.
	pane, err := NewGraphPanePostgres(pool).ForArtifact(ctx, artID)
	if err != nil {
		t.Fatalf("ForArtifact: %v", err)
	}
	if len(pane.Entities) != 1 || pane.Entities[0].ID != hit.ID || pane.Entities[0].Origin != "tag" {
		t.Fatalf("pane entities = %+v", pane.Entities)
	}

	// Detach removes it.
	if err := tagRepo.DetachTag(ctx, artID, hit.ID); err != nil {
		t.Fatalf("detach: %v", err)
	}
	attached, err = tagRepo.ListForArtifact(ctx, artID)
	if err != nil {
		t.Fatalf("list after detach: %v", err)
	}
	if len(attached) != 0 {
		t.Fatalf("expected no attached entities after detach, got %+v", attached)
	}
}

// TestEntityTag_ReplaceMentions covers the P2 projection: syncing @entity mentions
// reconciles origin='mention' rows (set replaces prior), without touching tags.
func TestEntityTag_ReplaceMentions(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artID := "men-art-" + uuid.NewString()
	ctx = adminContext(userID) // mention/tag writes now emit a node frame → need a principal

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'Mentions test', '')
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, artID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	tagRepo := NewEntityTagPostgres(pool)
	e1, _ := tagRepo.CreateEntity(ctx, "org", "Mco"+tag, entities.Normalize("Mco"+tag))
	e2, _ := tagRepo.CreateEntity(ctx, "person", "Pco"+tag, entities.Normalize("Pco"+tag))

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid, $2::uuid)`, e1.ID, e2.ID)
	})

	// A tag must survive mention syncs.
	if err := tagRepo.AttachTag(ctx, artID, e1.ID, userID); err != nil {
		t.Fatalf("attach tag: %v", err)
	}

	// First projection: e1 mentioned in block b1, e2 in b2.
	if err := tagRepo.ReplaceMentions(ctx, artID, []coreservice.MentionRef{
		{EntityID: e1.ID, BlockID: "b1", Surface: "Mco"},
		{EntityID: e2.ID, BlockID: "b2", Surface: "Pco"},
	}); err != nil {
		t.Fatalf("replace mentions: %v", err)
	}
	att, _ := tagRepo.ListForArtifact(ctx, artID)
	mentions := 0
	tags := 0
	for _, a := range att {
		if a.Origin == "mention" {
			mentions++
		}
		if a.Origin == "tag" {
			tags++
		}
	}
	if mentions != 2 || tags != 1 {
		t.Fatalf("after first sync: mentions=%d tags=%d (%+v)", mentions, tags, att)
	}

	// Re-sync with only e1: e2's mention drops, e1's tag stays.
	if err := tagRepo.ReplaceMentions(ctx, artID, []coreservice.MentionRef{
		{EntityID: e1.ID, BlockID: "b1", Surface: "Mco"},
	}); err != nil {
		t.Fatalf("replace mentions 2: %v", err)
	}
	att, _ = tagRepo.ListForArtifact(ctx, artID)
	mentions, tags = 0, 0
	for _, a := range att {
		if a.Origin == "mention" {
			mentions++
		}
		if a.Origin == "tag" {
			tags++
		}
	}
	if mentions != 1 || tags != 1 {
		t.Fatalf("after re-sync: mentions=%d tags=%d (%+v)", mentions, tags, att)
	}

	// Empty sync clears all mentions but keeps the tag.
	if err := tagRepo.ReplaceMentions(ctx, artID, nil); err != nil {
		t.Fatalf("replace mentions empty: %v", err)
	}
	att, _ = tagRepo.ListForArtifact(ctx, artID)
	if len(att) != 1 || att[0].Origin != "tag" {
		t.Fatalf("after empty sync expected only the tag, got %+v", att)
	}
}

// TestEntityTag_CreateEntityDedup covers Y0.1: the "create new" path is find-or-create.
// A second create with the same normalized key (e.g. the @-mention "create" path, which
// passes kind='unknown') reuses the existing shared entity instead of minting a duplicate,
// and the typed kind from the first create wins.
func TestEntityTag_CreateEntityDedup(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	name := "Dedupco" + tag
	key := entities.Normalize(name)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE normalized_key = $1`, key)
	})

	tagRepo := NewEntityTagPostgres(pool)

	// First create mints the entity (typed).
	first, err := tagRepo.CreateEntity(ctx, "org", name, key)
	if err != nil {
		t.Fatalf("create 1: %v", err)
	}
	if first.ID == "" {
		t.Fatalf("create 1 returned empty id")
	}

	// Second create with the same key but kind='unknown' reuses it rather than duplicating,
	// and keeps the typed kind.
	second, err := tagRepo.CreateEntity(ctx, "unknown", name, key)
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected dedup to reuse id %s, got %s", first.ID, second.ID)
	}
	if second.Kind != "org" {
		t.Fatalf("expected reused entity kind 'org', got %q", second.Kind)
	}

	// Exactly one shared row exists for this key.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM entities WHERE normalized_key = $1 AND scope = 'shared'`, key,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 shared entity row for key, got %d", count)
	}
}

// TestEntityTag_AliasAwareSearch covers P1.1: the typeahead matches ANY known surface
// (alias) of an entity, returns the synonym list on each hit, resolves merged-away
// entities to their canonical root (deduped), and still finds entities that have no
// alias rows at all (pre-00020-backfill rows, via the direct-match arm).
func TestEntityTag_AliasAwareSearch(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	rootName := "Nvidia" + tag
	aliasSurface := "NVDA" + tag
	mergedName := "NvidiaCorp" + tag

	var rootID, mergedID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key)
		VALUES ('shared', 'org', $1, $2) RETURNING id::text
	`, rootName, entities.Normalize(rootName)).Scan(&rootID); err != nil {
		t.Fatalf("seed root entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, canonical_root_id)
		VALUES ('shared', 'org', $1, $2, $3::uuid) RETURNING id::text
	`, mergedName, entities.Normalize(mergedName), rootID).Scan(&mergedID); err != nil {
		t.Fatalf("seed merged entity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_aliases (entity_id, surface, normalized) VALUES ($1::uuid, $2, $3)
	`, rootID, aliasSurface, entities.Normalize(aliasSurface)); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid, $2::uuid)`, mergedID, rootID)
	})

	tagRepo := NewEntityTagPostgres(pool)

	// 1. Searching by the ALIAS surface finds the entity; the hit carries the synonym.
	hits, err := tagRepo.SearchEntities(ctx, "", aliasSurface, 10)
	if err != nil {
		t.Fatalf("search by alias: %v", err)
	}
	if len(hits) == 0 || hits[0].ID != rootID {
		t.Fatalf("expected first hit %s for alias query, got %+v", rootID, hits)
	}
	foundAlias := false
	for _, a := range hits[0].Aliases {
		if a == aliasSurface {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Fatalf("expected hit to carry alias %q, got %v", aliasSurface, hits[0].Aliases)
	}

	// 2. The canonical name still matches even though it has NO alias row (direct arm).
	hits, err = tagRepo.SearchEntities(ctx, "", rootName, 10)
	if err != nil {
		t.Fatalf("search by canonical: %v", err)
	}
	if len(hits) == 0 || hits[0].ID != rootID {
		t.Fatalf("expected first hit %s for canonical query, got %+v", rootID, hits)
	}

	// 3. A query matching a merged-away entity resolves to the ROOT, deduped: the
	// merged-away id never appears.
	hits, err = tagRepo.SearchEntities(ctx, "", mergedName, 10)
	if err != nil {
		t.Fatalf("search by merged name: %v", err)
	}
	if len(hits) == 0 || hits[0].ID != rootID {
		t.Fatalf("expected merged-away query to resolve to root %s, got %+v", rootID, hits)
	}
	for _, h := range hits {
		if h.ID == mergedID {
			t.Fatalf("merged-away entity %s leaked into results: %+v", mergedID, hits)
		}
	}
}

// TestEntityTag_PlaceholderCreate covers P1.2: create is alias-aware exact find-or-create.
// A miss mints a PLACEHOLDER (trust_tier='placeholder') with its canonical alias seeded;
// a repeat create reuses the same placeholder; a name matching an existing entity's ALIAS
// reuses that entity; a name matching a merged-away entity resolves to its root.
func TestEntityTag_PlaceholderCreate(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	freshName := "Ghost" + tag
	aliasedName := "Alia" + tag
	aliasSurface := "Synon" + tag
	mergedName := "Oldco" + tag
	rootName := "Newco" + tag

	tagRepo := NewEntityTagPostgres(pool)

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE canonical_name IN ($1, $2, $3, $4)`,
			freshName, aliasedName, mergedName, rootName)
	})

	// 1. Miss → placeholder minted, canonical alias seeded.
	first, err := tagRepo.CreateEntity(ctx, "other", freshName, entities.Normalize(freshName))
	if err != nil {
		t.Fatalf("create placeholder: %v", err)
	}
	if first.TrustTier != "placeholder" {
		t.Fatalf("expected trust_tier placeholder, got %q", first.TrustTier)
	}
	var aliasCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM entity_aliases WHERE entity_id = $1::uuid AND normalized = $2`,
		first.ID, entities.Normalize(freshName)).Scan(&aliasCount); err != nil {
		t.Fatalf("alias count: %v", err)
	}
	if aliasCount != 1 {
		t.Fatalf("expected canonical alias seeded, count = %d", aliasCount)
	}

	// 2. Repeat create reuses the placeholder (no dupe one level up).
	again, err := tagRepo.CreateEntity(ctx, "other", freshName, entities.Normalize(freshName))
	if err != nil {
		t.Fatalf("repeat create: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("expected placeholder reuse %s, got %s", first.ID, again.ID)
	}

	// 3. A name matching an existing entity's ALIAS reuses that entity.
	var aliasedID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('shared', 'org', $1, $2, 'believed') RETURNING id::text
	`, aliasedName, entities.Normalize(aliasedName)).Scan(&aliasedID); err != nil {
		t.Fatalf("seed aliased entity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_aliases (entity_id, surface, normalized) VALUES ($1::uuid, $2, $3)
	`, aliasedID, aliasSurface, entities.Normalize(aliasSurface)); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
	viaAlias, err := tagRepo.CreateEntity(ctx, "other", aliasSurface, entities.Normalize(aliasSurface))
	if err != nil {
		t.Fatalf("create via alias: %v", err)
	}
	if viaAlias.ID != aliasedID {
		t.Fatalf("expected alias-aware dedup to reuse %s, got %s", aliasedID, viaAlias.ID)
	}

	// 4. A name matching a merged-away entity resolves to its canonical root.
	var rootID, mergedID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('shared', 'org', $1, $2, 'believed') RETURNING id::text
	`, rootName, entities.Normalize(rootName)).Scan(&rootID); err != nil {
		t.Fatalf("seed root: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, canonical_root_id)
		VALUES ('shared', 'org', $1, $2, $3::uuid) RETURNING id::text
	`, mergedName, entities.Normalize(mergedName), rootID).Scan(&mergedID); err != nil {
		t.Fatalf("seed merged: %v", err)
	}
	viaMerged, err := tagRepo.CreateEntity(ctx, "other", mergedName, entities.Normalize(mergedName))
	if err != nil {
		t.Fatalf("create via merged name: %v", err)
	}
	if viaMerged.ID != rootID {
		t.Fatalf("expected merged-away create to resolve to root %s, got %s", rootID, viaMerged.ID)
	}
}
