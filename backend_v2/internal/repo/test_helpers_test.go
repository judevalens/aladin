package repo

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testAdminUserID = uuid.NewString()
var testRunSuffix = uuid.NewString()[:8]

func tid(name string) string { return name + "-" + testRunSuffix }
func strptr(value string) *string { return &value }

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
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

func seedUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, created_at, updated_at)
		VALUES ($1::uuid, $2, now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID, userID+"@example.com")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func cleanupSyncTables(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, query := range []string{
		`DELETE FROM outbox_events WHERE user_id = $1::uuid`,
		`DELETE FROM tree_nodes WHERE user_id = $1::uuid`,
		`DELETE FROM artifacts WHERE user_id = $1::uuid`,
	} {
		if _, err := pool.Exec(ctx, query, testAdminUserID); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
}

func adminContext(userID string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:    userID,
		ActorType: auth.ActorTypeUserSession,
		ActorID:   userID,
		Scopes:    []string{auth.ScopeArtifactsRead, auth.ScopeArtifactsWrite},
	})
}
