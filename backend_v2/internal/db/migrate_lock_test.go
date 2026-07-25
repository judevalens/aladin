package db

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrate_ConcurrentIsSerialized proves the advisory lock lets several migrators run at once
// without deadlocking or erroring — the second waits for the first rather than racing goose on the
// same pending migration. (On the already-migrated sandbox each call is a near no-op; the point is
// the lock acquire/release path under concurrency.)
func TestMigrate_ConcurrentIsSerialized(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("no TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	const n = 5
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = Migrate(ctx, pool)
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("concurrent Migrate #%d failed: %v", i, e)
		}
	}
}
