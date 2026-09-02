package repo

import (
	"context"
	"testing"
	"time"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

func TestUpdateArtifactGraphCommitsMoveFieldsAndOutboxTogether(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctx, t)
	defer pool.Close()

	userID := uuid.NewString()
	artifactID := "aggregate-artifact-" + uuid.NewString()
	folderID := "aggregate-folder-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`, userID, "aggregate-"+uuid.NewString()[:8]+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID) })
	if _, err := pool.Exec(ctx, `INSERT INTO artifacts (id, user_id, type, title, content) VALUES ($1, $2::uuid, 'page', 'Before', '')`, artifactID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tree_nodes (id, user_id, kind, title, position) VALUES ($1, $2::uuid, 'folder', 'Target', 1)`, folderID, userID); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position) VALUES ($1, $2::uuid, 'artifact', $1, 2)`, artifactID, userID); err != nil {
		t.Fatalf("seed artifact node: %v", err)
	}

	title := "After"
	if err := NewArtifactsPostgres(pool).UpdateArtifactGraph(adminContext(userID), artifactID, artifactservice.ArtifactPatch{Title: &title, FolderID: &folderID}); err != nil {
		t.Fatalf("UpdateArtifactGraph: %v", err)
	}

	var gotTitle string
	var gotParent *string
	if err := pool.QueryRow(ctx, `SELECT a.title, n.parent_id FROM artifacts a JOIN tree_nodes n ON n.artifact_id = a.id WHERE a.id = $1`, artifactID).Scan(&gotTitle, &gotParent); err != nil {
		t.Fatalf("read aggregate: %v", err)
	}
	if gotTitle != title || gotParent == nil || *gotParent != folderID {
		t.Fatalf("aggregate = title %q parent %#v, want %q/%q", gotTitle, gotParent, title, folderID)
	}

	var events, transactions int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(DISTINCT xid::text) FROM outbox_events WHERE user_id = $1::uuid`, userID).Scan(&events, &transactions); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if events != 2 || transactions != 1 {
		t.Fatalf("outbox = %d events across %d transactions, want 2 events in 1 transaction", events, transactions)
	}
}
