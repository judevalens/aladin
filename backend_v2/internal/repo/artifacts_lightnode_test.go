package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestLightNodeScansAllColumns guards the read-back inside ArtifactService.Create. lightEntitySelect
// projects 14 columns (…, summary, metadata, run_state, exec_mode, source_kind, seq, is_deleted);
// LightNode must scan them all — omitting any triggers pgx's "number of field descriptions must
// equal number of destinations", which failed every copilot/MCP create. Widening
// lightEntitySelect without widening LightNode's scan is the exact regression this catches.
func TestLightNodeScansAllColumns(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	userID := uuid.NewString()
	artID := "ln-art-" + uuid.NewString()
	ctx = adminContext(userID)

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "ln-"+uuid.NewString()[:8]+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, summary, metadata)
		VALUES ($1, $2::uuid, 'app', 'Shard', '', 'a summary', '{"agent":{"source":"copilot"}}'::jsonb)
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, artID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tree_nodes WHERE id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	node, err := NewArtifactsPostgres(pool).LightNode(ctx, artID)
	if err != nil {
		t.Fatalf("LightNode: %v", err)
	}
	if node.ID != artID || node.Kind != "artifact" {
		t.Fatalf("unexpected node: %+v", node)
	}
}
