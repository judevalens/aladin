package postgres_test

import (
	"context"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	systempostgres "aladin/backend_v2/internal/system/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestWorkerStatus_HeartbeatFreshness proves WorkerStatus reports real queue stats + liveness from
// the worker heartbeat (not fabricated zeros): a fresh beat → workerUp + stats surface; a stale
// beat → workerUp=false.
func TestWorkerStatus_HeartbeatFreshness(t *testing.T) {
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
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		// Restore the stale seed so other reads aren't affected.
		_, _ = pool.Exec(context.Background(),
			`UPDATE worker_heartbeat SET updated_at = now() - interval '1 hour', stats = '{}'::jsonb WHERE id = 1`)
	})

	sys := systempostgres.NewSystemPostgres(pool)

	// Fresh heartbeat with real counts.
	if _, err := pool.Exec(ctx, `UPDATE worker_heartbeat SET updated_at = now(), stats = '{"failed":3,"active":2}'::jsonb WHERE id = 1`); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}
	st, err := sys.WorkerStatus(ctx)
	if err != nil {
		t.Fatalf("WorkerStatus: %v", err)
	}
	if up, _ := st["workerUp"].(bool); !up {
		t.Fatalf("workerUp = %v, want true on a fresh heartbeat", st["workerUp"])
	}
	queue, _ := st["queue"].(map[string]any)
	if queue["failed"] != float64(3) {
		t.Fatalf("queue.failed = %v (%T), want 3 — real stats not surfaced", queue["failed"], queue["failed"])
	}

	// Stale heartbeat → worker reported down.
	if _, err := pool.Exec(ctx, `UPDATE worker_heartbeat SET updated_at = now() - interval '10 minutes' WHERE id = 1`); err != nil {
		t.Fatalf("stale heartbeat: %v", err)
	}
	st2, err := sys.WorkerStatus(ctx)
	if err != nil {
		t.Fatalf("WorkerStatus stale: %v", err)
	}
	if up, _ := st2["workerUp"].(bool); up {
		t.Fatal("workerUp = true on a 10-min-old heartbeat, want false")
	}
}
