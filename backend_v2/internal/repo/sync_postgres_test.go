package repo

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testAdminUserID = "00000000-0000-0000-0000-000000000001"

func strptr(s string) *string { return &s }

// Exercises the change-feed core: append field-level rows under the per-user
// lock, then pull a coalesced delta. Skips if no Postgres is reachable.
func TestSyncChangeFeed_AppendAndCoalescedPull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aladin:password@localhost:5433/aladin?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewSyncPostgres(pool)
	const entityID = "test-sync-node-A"

	// Clean slate for this entity, and restore it after.
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`,
			testAdminUserID, entityID)
	}
	cleanup()
	defer cleanup()

	// Cursor before our writes: pull-since-this returns only our appends.
	var cursor0 int64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM workspace_changes WHERE user_id = $1`,
		testAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	// One write txn: lock the user, append three field-level changes.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := LockUser(ctx, tx, testAdminUserID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	mut := "client-1:1"
	appends := []Change{
		{EntityKind: "node", EntityID: entityID, Op: OpUpdate, Field: strptr("title"), Value: json.RawMessage(`"v1"`), MutationID: &mut},
		{EntityKind: "node", EntityID: entityID, Op: OpUpdate, Field: strptr("title"), Value: json.RawMessage(`"v2"`), MutationID: &mut},
		{EntityKind: "node", EntityID: entityID, Op: OpUpdate, Field: strptr("body"), Value: json.RawMessage(`"x"`), MutationID: &mut},
	}
	var lastSeq int64
	for _, c := range appends {
		seq, err := AppendChange(ctx, tx, testAdminUserID, c)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("append: %v", err)
		}
		lastSeq = seq
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Pull the delta: title coalesced to v2 + body=x → 2 changes, seq-ordered.
	res, err := repo.PullDelta(ctx, testAdminUserID, cursor0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("coalesce failed: got %d changes, want 2 (title latest + body): %+v", len(res.Changes), res.Changes)
	}
	// seq order
	if res.Changes[0].Seq >= res.Changes[1].Seq {
		t.Fatalf("not seq-ordered: %+v", res.Changes)
	}
	// title resolves to the latest value (v2), not v1
	var sawTitleV2, sawBody bool
	for _, c := range res.Changes {
		if c.Field != nil && *c.Field == "title" {
			if strings.TrimSpace(string(c.Value)) != `"v2"` {
				t.Fatalf("title coalesced to %s, want \"v2\"", string(c.Value))
			}
			sawTitleV2 = true
		}
		if c.Field != nil && *c.Field == "body" {
			sawBody = true
		}
	}
	if !sawTitleV2 || !sawBody {
		t.Fatalf("missing expected fields: title-v2=%v body=%v", sawTitleV2, sawBody)
	}
	// cursor advanced to the feed high-water
	if res.Cursor != lastSeq {
		t.Fatalf("cursor = %d, want high-water %d", res.Cursor, lastSeq)
	}

	// A pull from the new cursor returns nothing and holds the cursor.
	res2, err := repo.PullDelta(ctx, testAdminUserID, res.Cursor)
	if err != nil {
		t.Fatalf("pull2: %v", err)
	}
	if len(res2.Changes) != 0 || res2.Cursor != res.Cursor {
		t.Fatalf("idempotent re-pull failed: %+v", res2)
	}
}

// Full loop: rename a folder via the repo → the change feed gets a
// folder/title row → PullDelta returns it. Proves AppendChange is wired into a
// real mutation in-txn under LockUser.
func TestSyncChangeFeed_FolderRenameEmitsChange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aladin:password@localhost:5433/aladin?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const folderID = "test-sync-folder-rename"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`, testAdminUserID, folderID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE user_id = $1::uuid AND id = $2`, testAdminUserID, folderID)
	}
	cleanup()
	defer cleanup()

	// Seed a folder node directly (no change row — only the rename should emit one).
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, NULL, 'folder', 'Old', NULL, 9000001, now(), now())
	`, folderID, testAdminUserID); err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	var cursor0 int64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM workspace_changes WHERE user_id = $1`,
		testAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	r := NewArtifactsPostgres(pool)
	pctx := coreservice.WithPrincipal(ctx, coreservice.Principal{
		UserID: testAdminUserID, ActorType: coreservice.ActorTypeUserSession, ActorID: testAdminUserID,
	})
	if err := r.UpdateFolderTitle(pctx, folderID, "New"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	res, err := NewSyncPostgres(pool).PullDelta(ctx, testAdminUserID, cursor0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(res.Changes) != 1 {
		t.Fatalf("changes = %d, want 1: %+v", len(res.Changes), res.Changes)
	}
	c := res.Changes[0]
	if c.EntityKind != "folder" || c.EntityID != folderID || c.Op != OpUpdate ||
		c.Field == nil || *c.Field != "title" || strings.TrimSpace(string(c.Value)) != `"New"` {
		t.Fatalf("unexpected change: %+v (value=%s)", c, string(c.Value))
	}
}

// Artifact create emits create + core fields; delete emits a tombstone.
func TestSyncChangeFeed_ArtifactCreateAndDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aladin:password@localhost:5433/aladin?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const artifactID = "test-sync-artifact-A"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`, testAdminUserID, artifactID)
		_, _ = pool.Exec(ctx, `DELETE FROM artifacts WHERE id = $1 AND user_id = $2::uuid`, artifactID, testAdminUserID)
		_, _ = pool.Exec(ctx, `DELETE FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid`, artifactID, testAdminUserID)
	}
	cleanup()
	defer cleanup()

	r := NewArtifactsPostgres(pool)
	sync := NewSyncPostgres(pool)
	pctx := coreservice.WithPrincipal(ctx, coreservice.Principal{
		UserID: testAdminUserID, ActorType: coreservice.ActorTypeUserSession, ActorID: testAdminUserID,
	})

	var cursor0 int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(seq),0) FROM workspace_changes WHERE user_id=$1`, testAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	rec := coreservice.ArtifactResponse{ID: artifactID, Type: "note", Title: "Hello", Content: "", Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now}
	node := coreservice.TreeNodeRecord{ID: artifactID, ParentID: nil, Kind: "artifact", Title: nil, ArtifactID: strptr(artifactID), Position: 9000002}
	if err := r.CreateArtifactGraph(pctx, rec, node, nil, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	res, err := sync.PullDelta(ctx, testAdminUserID, cursor0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	var sawCreate bool
	fields := map[string]string{}
	for _, c := range res.Changes {
		if c.EntityKind != "artifact" || c.EntityID != artifactID {
			t.Fatalf("unexpected change entity: %+v", c)
		}
		if c.Op == OpCreate {
			sawCreate = true
		} else if c.Field != nil {
			fields[*c.Field] = strings.TrimSpace(string(c.Value))
		}
	}
	if !sawCreate {
		t.Fatalf("missing create op: %+v", res.Changes)
	}
	if fields["title"] != `"Hello"` || fields["type"] != `"note"` {
		t.Fatalf("create fields wrong: %v", fields)
	}
	if _, ok := fields["parentId"]; !ok {
		t.Fatalf("missing parentId field: %v", fields)
	}

	// Delete → tombstone.
	if err := r.DeleteArtifact(pctx, artifactID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	res2, err := sync.PullDelta(ctx, testAdminUserID, res.Cursor)
	if err != nil {
		t.Fatalf("pull2: %v", err)
	}
	if len(res2.Changes) != 1 || res2.Changes[0].Op != OpDelete || res2.Changes[0].EntityID != artifactID {
		t.Fatalf("expected single delete tombstone, got: %+v", res2.Changes)
	}
}
