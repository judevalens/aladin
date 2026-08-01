package repo

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"

	"github.com/google/uuid"
)

// TestGraphPane_ForArtifact assembles the pane for an artifact you're viewing: the entities
// connected to it via artifact_entities (tag / @mention), each with how many times it is
// mentioned across records. The claim layer was removed, so the pane is entity-only.
func TestGraphPane_ForArtifact(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	entName := "acmecorp" + tag
	userID := uuid.NewString()
	artID := "gp-art-" + uuid.NewString()
	r1 := "gp-ar1-" + uuid.NewString()
	r2 := "gp-ar2-" + uuid.NewString()

	// artifact_entities FKs the artifact, which FKs the user.
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'Pane test '||$3, '')
	`, artID, userID, tag); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	entityRepo := db.NewEntityRepository(pool)

	eid, err := entityRepo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "org", CanonicalName: entName, NormalizedKey: entName})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	// Two records mention the entity — that's the mention count the pane reports.
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

	// Connect the entity to the artifact you're viewing (the tag path).
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifact_entities (artifact_id, entity_id, origin, added_by)
		VALUES ($1, $2::uuid, 'tag', $3::uuid)
		ON CONFLICT DO NOTHING
	`, artID, eid, userID); err != nil {
		t.Fatalf("tag artifact: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM artifact_entities WHERE artifact_id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1::uuid`, eid)
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE id IN ($1, $2)`, r1, r2)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	pane, err := NewGraphPanePostgres(pool).ForArtifact(ctx, artID)
	if err != nil {
		t.Fatalf("ForArtifact: %v", err)
	}
	if len(pane.Entities) != 1 {
		t.Fatalf("entities = %+v, want exactly the tagged entity", pane.Entities)
	}
	got := pane.Entities[0]
	if got.ID != eid || got.Kind != "org" || got.Origin != "tag" || got.Mentions != 2 {
		t.Fatalf("entity = %+v, want id=%s kind=org origin=tag mentions=2", got, eid)
	}
}
