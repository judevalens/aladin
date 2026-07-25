package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Unique per test process — see the note in internal/repo/sync_postgres_test.go. This was the same
// hardcoded id as internal/repo + internal/mcp, so those packages (run in parallel against the one
// shared sandbox DB) clobbered each other's sync rows.
var syncTestAdminUserID = uuid.NewString()

// The pull endpoint requires an authenticated principal (checked before any
// service call, so a nil Sync() is never reached here).
func TestHandleSyncPull_RequiresPrincipal(t *testing.T) {
	s := &Server{deps: app.StaticDependencies{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sync/pull", nil)
	s.handleSyncPull(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-principal status = %d, want 401", rec.Code)
	}
}

// A non-numeric ?since cursor is a bad request (uint64 decimal string expected).
func TestHandleSyncPull_RejectsBadCursor(t *testing.T) {
	ctx := coreservice.WithPrincipal(context.Background(), coreservice.Principal{
		UserID: syncTestAdminUserID,
		Scopes: []string{string(coreservice.ScopeArtifactsRead)},
	})
	s := &Server{deps: app.StaticDependencies{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sync/pull?since=not-a-number", nil).WithContext(ctx)
	s.handleSyncPull(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad-cursor status = %d, want 400", rec.Code)
	}
}

// End-to-end: a producer write (create folder) lands in the outbox; a cold pull
// (since=0) returns it as a snapshot; pulling again from the returned cursor is
// empty (incremental). DB integration test — skips without Postgres.
func TestHandleSyncPull_ReturnsFramesAndAdvancesCursor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := mustSyncTestPool(ctx, t)
	defer pool.Close()
	// Scoped to this test's user — an unscoped delete wipes rows other packages are asserting on
	// (shared sandbox DB, parallel package processes). Separate Execs: a parameterised statement
	// can't carry multiple commands.
	for _, q := range []string{
		`DELETE FROM outbox_events WHERE user_id = $1::uuid`,
		`DELETE FROM tree_nodes WHERE user_id = $1::uuid`,
		`DELETE FROM artifacts WHERE user_id = $1::uuid`,
	} {
		if _, err := pool.Exec(ctx, q, syncTestAdminUserID); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, created_at, updated_at)
		VALUES ($1::uuid, $2, now(), now()) ON CONFLICT (id) DO NOTHING
	`, syncTestAdminUserID, syncTestAdminUserID+"@example.com"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	principalCtx := coreservice.WithPrincipal(ctx, coreservice.Principal{
		UserID: syncTestAdminUserID,
		Scopes: []string{string(coreservice.ScopeArtifactsRead), string(coreservice.ScopeArtifactsWrite)},
	})

	// Producer: create a folder (emits one outbox frame). The id is namespaced to this test process
	// because tree_nodes.id is a GLOBAL primary key — a fixed "folder-a" collides with the same
	// literal in internal/repo's parallel run and with rows left by earlier runs.
	title := "Folder A"
	if err := repo.NewArtifactsPostgres(pool).CreateTreeNode(principalCtx, coreservice.TreeNodeRecord{
		ID: "folder-a-" + uuid.NewString()[:8], Kind: "folder", Title: &title, Position: 1,
	}); err != nil {
		t.Fatalf("create tree node: %v", err)
	}

	deps := app.StaticDependencies{
		SyncSvc: coreservice.NewSyncService(repo.NewSyncPostgres(pool), repo.NewTreeSyncSource(pool)),
	}
	s := &Server{deps: deps}

	// Cold pull (since=0) → snapshot containing the folder.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sync/pull?since=0", nil).WithContext(principalCtx)
	s.handleSyncPull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pull status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var res coreservice.PullResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode pull: %v (body: %s)", err, rec.Body.String())
	}
	if res.Mode != coreservice.PullModeSnapshot {
		t.Fatalf("mode = %q, want snapshot", res.Mode)
	}
	if n := countEntities(res.Frames); n != 1 {
		t.Fatalf("snapshot entity count = %d, want 1", n)
	}
	if res.Cursor == 0 {
		t.Fatalf("cursor = 0, want > 0")
	}

	// Incremental pull from the returned cursor → nothing new.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet,
		"/api/sync/pull?since="+strconv.FormatUint(res.Cursor, 10), nil).WithContext(principalCtx)
	s.handleSyncPull(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("pull2 status = %d, want 200", rec2.Code)
	}
	var res2 coreservice.PullResult
	if err := json.Unmarshal(rec2.Body.Bytes(), &res2); err != nil {
		t.Fatalf("decode pull2: %v", err)
	}
	if res2.Mode != coreservice.PullModeDelta {
		t.Fatalf("mode2 = %q, want delta", res2.Mode)
	}
	if n := countEntities(res2.Frames); n != 0 {
		t.Fatalf("incremental pull returned %d entities, want 0", n)
	}
}

func countEntities(frames []coreservice.Frame) int {
	n := 0
	for _, f := range frames {
		n += len(f.Entities)
	}
	return n
}

func mustSyncTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	// SAFETY: this test wipes workspace tables — only ever run it against an
	// explicit throwaway TEST_DATABASE_URL, never the dev DB.
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
