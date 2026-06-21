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

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'History test', '')
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
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

	// Typeahead finds it.
	hits, err := tagRepo.SearchEntities(ctx, userID, "Tagco", 10)
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

	// It surfaces in the graph pane as a tag-origin entity (no claims needed).
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

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'Mentions test', '')
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
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
