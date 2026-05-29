package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

const syncTestAdminUserID = "00000000-0000-0000-0000-000000000001"

func TestHandleSyncPull_RequiresPrincipal(t *testing.T) {
	s := &Server{deps: app.StaticDependencies{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sync/pull", nil)
	s.handleSyncPull(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-principal status = %d, want 401", rec.Code)
	}
}

func TestHandleSyncPull_ReturnsDelta(t *testing.T) {
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

	const entityID = "test-sync-pull-handler"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`,
			syncTestAdminUserID, entityID)
	}
	cleanup()
	defer cleanup()

	var cursor0 int64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM workspace_changes WHERE user_id = $1`,
		syncTestAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	// Seed one change row.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := repo.LockUser(ctx, tx, syncTestAdminUserID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	field := "title"
	if _, err := repo.AppendChange(ctx, tx, syncTestAdminUserID, repo.Change{
		EntityKind: "folder", EntityID: entityID, Op: repo.OpUpdate,
		Field: &field, Value: json.RawMessage(`"Renamed"`),
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("append: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Hit the handler with an authenticated principal in context.
	s := &Server{deps: app.StaticDependencies{SyncRepo: repo.NewSyncPostgres(pool)}}
	principal := coreservice.Principal{
		UserID:    syncTestAdminUserID,
		ActorType: coreservice.ActorTypeUserSession,
		ActorID:   syncTestAdminUserID,
	}
	reqCtx := coreservice.WithPrincipal(ctx, principal)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/sync/pull?since="+strconv.FormatInt(cursor0, 10), nil).WithContext(reqCtx)
	s.handleSyncPull(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var res repo.PullResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, rec.Body.String())
	}
	if len(res.Changes) != 1 {
		t.Fatalf("changes = %d, want 1: %+v", len(res.Changes), res.Changes)
	}
	c := res.Changes[0]
	if c.EntityKind != "folder" || c.EntityID != entityID || c.Field == nil || *c.Field != "title" {
		t.Fatalf("unexpected change: %+v", c)
	}
	if res.Cursor <= cursor0 {
		t.Fatalf("cursor did not advance: %d <= %d", res.Cursor, cursor0)
	}
}
