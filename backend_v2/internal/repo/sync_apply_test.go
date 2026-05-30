package repo

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Shared setup for the generic write-path tests. Skips without Postgres.
func applyTestPool(t *testing.T) (*pgxpool.Pool, context.Context, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aladin:password@localhost:5433/aladin?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		cancel()
		t.Skipf("postgres not reachable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		cancel()
		t.Skipf("postgres ping failed: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		cancel()
		t.Fatalf("migrate: %v", err)
	}
	return pool, ctx, func() { pool.Close(); cancel() }
}

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestApplyMutation_CreateFolderIdempotent(t *testing.T) {
	pool, ctx, done := applyTestPool(t)
	defer done()

	const folderID = "test-apply-folder"
	const clientID = "test-apply-client-folder"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`, testAdminUserID, folderID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id = $1::uuid AND id = $2`, testAdminUserID, folderID)
		_, _ = pool.Exec(ctx, `DELETE FROM sync_clients WHERE client_id = $1`, clientID)
	}
	cleanup()
	defer cleanup()

	repo := NewSyncPostgres(pool)
	var cursor0 int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(seq),0) FROM workspace_changes WHERE user_id=$1`, testAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	m := Mutation{
		MutationID: clientID + ":1",
		Op:         OpCreate,
		EntityKind: "folder",
		EntityID:   folderID,
		Title:      strptr("Docs"),
		Position:   int64ptr(9000001),
	}
	ack, err := repo.ApplyMutation(ctx, testAdminUserID, m)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if ack.Status != ackApplied {
		t.Fatalf("status = %q, want applied", ack.Status)
	}

	// Canonical row written.
	var title string
	var pos int64
	if err := pool.QueryRow(ctx, `SELECT title, position FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid AND kind = 'folder'`,
		folderID, testAdminUserID).Scan(&title, &pos); err != nil {
		t.Fatalf("read tree_node: %v", err)
	}
	if title != "Docs" || pos <= 0 {
		t.Fatalf("tree_node = (%q,%d), want (Docs, server-assigned position>0)", title, pos)
	}

	// Feed has create + title + parentId + position.
	res, err := repo.PullDelta(ctx, testAdminUserID, cursor0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	var sawCreate bool
	fields := map[string]string{}
	for _, c := range res.Changes {
		if c.Op == OpCreate {
			sawCreate = true
		} else if c.Field != nil {
			fields[*c.Field] = strings.TrimSpace(string(c.Value))
		}
	}
	if !sawCreate || fields["title"] != `"Docs"` {
		t.Fatalf("feed wrong: create=%v fields=%v", sawCreate, fields)
	}
	if p, ok := fields["position"]; !ok || p == "" {
		t.Fatalf("feed missing server-assigned position: %v", fields)
	}

	// Re-applying the SAME mutationId is a duplicate no-op (idempotent).
	ack2, err := repo.ApplyMutation(ctx, testAdminUserID, m)
	if err != nil {
		t.Fatalf("apply dup: %v", err)
	}
	if ack2.Status != ackDuplicate {
		t.Fatalf("status = %q, want duplicate", ack2.Status)
	}
	res2, err := repo.PullDelta(ctx, testAdminUserID, res.Cursor)
	if err != nil {
		t.Fatalf("pull2: %v", err)
	}
	if len(res2.Changes) != 0 {
		t.Fatalf("duplicate produced %d new changes, want 0", len(res2.Changes))
	}
}

func TestApplyMutation_UpdateFolderTitle(t *testing.T) {
	pool, ctx, done := applyTestPool(t)
	defer done()

	const folderID = "test-apply-rename"
	const clientID = "test-apply-client-rename"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`, testAdminUserID, folderID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id = $1::uuid AND id = $2`, testAdminUserID, folderID)
		_, _ = pool.Exec(ctx, `DELETE FROM sync_clients WHERE client_id = $1`, clientID)
	}
	cleanup()
	defer cleanup()

	repo := NewSyncPostgres(pool)
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":1", Op: OpCreate, EntityKind: "folder", EntityID: folderID, Title: strptr("Old"), Position: int64ptr(9000004),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":2", Op: OpUpdate, EntityKind: "folder", EntityID: folderID,
		Field: strptr("title"), Value: rawJSON(t, "New"),
	}); err != nil {
		t.Fatalf("rename: %v", err)
	}

	var title string
	if err := pool.QueryRow(ctx, `SELECT title FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid`, folderID, testAdminUserID).Scan(&title); err != nil {
		t.Fatalf("read: %v", err)
	}
	if title != "New" {
		t.Fatalf("title = %q, want New", title)
	}
}

func TestApplyMutation_CreateArtifactAndDelete(t *testing.T) {
	pool, ctx, done := applyTestPool(t)
	defer done()

	const artifactID = "test-apply-artifact"
	const clientID = "test-apply-client-artifact"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`, testAdminUserID, artifactID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id = $1::uuid AND (id = $2 OR artifact_id = $2)`, testAdminUserID, artifactID)
		_, _ = pool.Exec(ctx, `DELETE FROM artifacts WHERE user_id = $1::uuid AND id = $2`, testAdminUserID, artifactID)
		_, _ = pool.Exec(ctx, `DELETE FROM sync_clients WHERE client_id = $1`, clientID)
	}
	cleanup()
	defer cleanup()

	repo := NewSyncPostgres(pool)
	var cursor0 int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(seq),0) FROM workspace_changes WHERE user_id=$1`, testAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":1", Op: OpCreate, EntityKind: "artifact", EntityID: artifactID,
		Title: strptr("Note A"), ArtifactType: strptr("note"), Position: int64ptr(9000002),
	}); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	// Both canonical rows exist.
	var typ string
	if err := pool.QueryRow(ctx, `SELECT type FROM artifacts WHERE id = $1 AND user_id = $2::uuid`, artifactID, testAdminUserID).Scan(&typ); err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if typ != "note" {
		t.Fatalf("type = %q, want note", typ)
	}
	var nodeCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tree_nodes WHERE artifact_id = $1 AND user_id = $2::uuid AND kind = 'artifact'`, artifactID, testAdminUserID).Scan(&nodeCount); err != nil {
		t.Fatalf("count node: %v", err)
	}
	if nodeCount != 1 {
		t.Fatalf("tree_node count = %d, want 1", nodeCount)
	}

	res, err := repo.PullDelta(ctx, testAdminUserID, cursor0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	fields := map[string]string{}
	var sawCreate bool
	for _, c := range res.Changes {
		if c.Op == OpCreate {
			sawCreate = true
		} else if c.Field != nil {
			fields[*c.Field] = strings.TrimSpace(string(c.Value))
		}
	}
	if !sawCreate || fields["title"] != `"Note A"` || fields["type"] != `"note"` {
		t.Fatalf("create feed wrong: create=%v fields=%v", sawCreate, fields)
	}

	// Delete → tombstone, both canonical rows gone.
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":2", Op: OpDelete, EntityKind: "artifact", EntityID: artifactID,
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM artifacts WHERE id = $1 AND user_id = $2::uuid`, artifactID, testAdminUserID).Scan(&remaining); err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("artifact remains after delete")
	}
	res2, err := repo.PullDelta(ctx, testAdminUserID, res.Cursor)
	if err != nil {
		t.Fatalf("pull2: %v", err)
	}
	if len(res2.Changes) != 1 || res2.Changes[0].Op != OpDelete {
		t.Fatalf("expected single delete tombstone, got %+v", res2.Changes)
	}
}

func TestApplyMutation_ArtifactMove(t *testing.T) {
	pool, ctx, done := applyTestPool(t)
	defer done()

	const artifactID = "test-apply-move"
	const parentID = "test-apply-move-parent"
	const clientID = "test-apply-client-move"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id IN ($2, $3)`, testAdminUserID, artifactID, parentID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id = $1::uuid AND (id = $2 OR artifact_id = $2 OR id = $3)`, testAdminUserID, artifactID, parentID)
		_, _ = pool.Exec(ctx, `DELETE FROM artifacts WHERE user_id = $1::uuid AND id = $2`, testAdminUserID, artifactID)
		_, _ = pool.Exec(ctx, `DELETE FROM sync_clients WHERE client_id = $1`, clientID)
	}
	cleanup()
	defer cleanup()

	repo := NewSyncPostgres(pool)
	// A real destination folder — tree_nodes.parent_id has an FK, so moves to a
	// non-existent parent are (correctly) rejected by the server.
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":1", Op: OpCreate, EntityKind: "folder", EntityID: parentID, Title: strptr("Dest"), Position: int64ptr(9000010),
	}); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":2", Op: OpCreate, EntityKind: "artifact", EntityID: artifactID, Title: strptr("Movable"), Position: int64ptr(9000003),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":3", Op: OpUpdate, EntityKind: "artifact", EntityID: artifactID,
		Field: strptr("parentId"), Value: rawJSON(t, parentID),
	}); err != nil {
		t.Fatalf("move: %v", err)
	}

	var parent *string
	if err := pool.QueryRow(ctx, `SELECT parent_id FROM tree_nodes WHERE artifact_id = $1 AND user_id = $2::uuid`, artifactID, testAdminUserID).Scan(&parent); err != nil {
		t.Fatalf("read parent: %v", err)
	}
	if parent == nil || *parent != parentID {
		t.Fatalf("parent = %v, want %s", parent, parentID)
	}
}

// Regression for the live two-client bug: a client computes its position hint
// from incomplete local state, so two creates can send the SAME colliding hint;
// the server must assign distinct non-colliding positions, not reject.
func TestApplyMutation_CreateAssignsNonCollidingPosition(t *testing.T) {
	pool, ctx, done := applyTestPool(t)
	defer done()

	const parentID = "test-apply-pos-parent"
	const a1 = "test-apply-pos-a1"
	const a2 = "test-apply-pos-a2"
	const clientID = "test-apply-pos-client"
	cleanup := func() {
		for _, id := range []string{parentID, a1, a2} {
			_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id=$1 AND entity_id=$2`, testAdminUserID, id)
			_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id=$1::uuid AND (id=$2 OR artifact_id=$2)`, testAdminUserID, id)
			_, _ = pool.Exec(ctx, `DELETE FROM artifacts WHERE user_id=$1::uuid AND id=$2`, testAdminUserID, id)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM sync_clients WHERE client_id=$1`, clientID)
	}
	cleanup()
	defer cleanup()

	repo := NewSyncPostgres(pool)
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":1", Op: OpCreate, EntityKind: "folder", EntityID: parentID, Title: strptr("Bin"),
	}); err != nil {
		t.Fatalf("parent: %v", err)
	}
	// Two artifacts under the same parent, BOTH sending the same colliding hint.
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":2", Op: OpCreate, EntityKind: "artifact", EntityID: a1, Title: strptr("A1"), Position: int64ptr(1),
	}); err != nil {
		t.Fatalf("a1: %v", err)
	}
	if _, err := repo.ApplyMutation(ctx, testAdminUserID, Mutation{
		MutationID: clientID + ":3", Op: OpCreate, EntityKind: "artifact", EntityID: a2, Title: strptr("A2"), Position: int64ptr(1),
	}); err != nil {
		t.Fatalf("a2 (same colliding position hint must still succeed): %v", err)
	}

	var p1, p2 int64
	if err := pool.QueryRow(ctx, `SELECT position FROM tree_nodes WHERE artifact_id=$1 AND user_id=$2::uuid`, a1, testAdminUserID).Scan(&p1); err != nil {
		t.Fatalf("p1: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT position FROM tree_nodes WHERE artifact_id=$1 AND user_id=$2::uuid`, a2, testAdminUserID).Scan(&p2); err != nil {
		t.Fatalf("p2: %v", err)
	}
	if p1 == p2 || p1 <= 0 || p2 <= 0 {
		t.Fatalf("positions must be distinct + positive, got p1=%d p2=%d", p1, p2)
	}
}

// A cold pull (cursor 0) must return a SNAPSHOT of the current canonical
// workspace — including entities that have NO change-feed rows (created before
// the feed existed).
func TestPullDelta_SnapshotIncludesPreFeedNodes(t *testing.T) {
	pool, ctx, done := applyTestPool(t)
	defer done()

	const folderID = "test-snap-folder"
	const artID = "test-snap-artifact"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id=$1 AND entity_id IN ($2,$3)`, testAdminUserID, folderID, artID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id=$1::uuid AND (id IN ($2,$3) OR artifact_id=$3)`, testAdminUserID, folderID, artID)
		_, _ = pool.Exec(ctx, `DELETE FROM artifacts WHERE user_id=$1::uuid AND id=$2`, testAdminUserID, artID)
	}
	cleanup()
	defer cleanup()

	// Seed canonical rows directly with NO feed rows (simulates pre-feed data).
	if _, err := pool.Exec(ctx, `INSERT INTO tree_nodes (id,user_id,parent_id,kind,title,artifact_id,position,created_at,updated_at)
		VALUES ($1,$2::uuid,NULL,'folder','PreFeed',NULL,9100001,now(),now())`, folderID, testAdminUserID); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO artifacts (id,user_id,type,title,content,metadata,created_at,updated_at)
		VALUES ($1,$2::uuid,'note','PreNote','','{}'::jsonb,now(),now())`, artID, testAdminUserID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tree_nodes (id,user_id,parent_id,kind,title,artifact_id,position,created_at,updated_at)
		VALUES ($1,$2::uuid,$3,'artifact',NULL,$1,9100002,now(),now())`, artID, testAdminUserID, folderID); err != nil {
		t.Fatalf("seed artifact node: %v", err)
	}

	res, err := NewSyncPostgres(pool).PullDelta(ctx, testAdminUserID, 0)
	if err != nil {
		t.Fatalf("pull snapshot: %v", err)
	}

	sawFolderCreate, sawArtCreate := false, false
	folderTitle, artType := "", ""
	for _, c := range res.Changes {
		switch {
		case c.EntityID == folderID && c.Op == OpCreate:
			sawFolderCreate = true
		case c.EntityID == artID && c.Op == OpCreate:
			sawArtCreate = true
		case c.EntityID == folderID && c.Field != nil && *c.Field == "title":
			folderTitle = strings.TrimSpace(string(c.Value))
		case c.EntityID == artID && c.Field != nil && *c.Field == "type":
			artType = strings.TrimSpace(string(c.Value))
		}
	}
	if !sawFolderCreate || !sawArtCreate {
		t.Fatalf("snapshot missing creates: folder=%v artifact=%v", sawFolderCreate, sawArtCreate)
	}
	if folderTitle != `"PreFeed"` {
		t.Fatalf("folder title = %s, want \"PreFeed\"", folderTitle)
	}
	if artType != `"note"` {
		t.Fatalf("artifact type = %s, want \"note\"", artType)
	}
}

func int64ptr(v int64) *int64 { return &v }
