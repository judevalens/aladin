package postgres

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

func adminContext(userID string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: userID, ActorType: auth.ActorTypeUserSession, ActorID: userID,
		Scopes: []string{auth.ScopeArtifactsRead, auth.ScopeArtifactsWrite},
	})
}
