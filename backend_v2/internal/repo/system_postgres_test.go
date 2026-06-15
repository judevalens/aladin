package repo

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestPipelineStats checks the read-only ingestion snapshot: valid SQL, the
// expected shape, and that a seeded record shows up in the by-status counts.
// Counts are global (shared DB), so it asserts ">= seeded" + structure, not totals.
func TestPipelineStats(t *testing.T) {
	t.Parallel()
	dsn := dbtest.RequireTestDSN(t)

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	recID := "stats-rec-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO records(id,type,label,content,status) VALUES($1,'post','l','c','captured')`, recID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id=$1`, recID)
	})

	stats, err := NewSystemPostgres(pool).PipelineStats(ctx)
	if err != nil {
		t.Fatalf("PipelineStats: %v", err)
	}

	records, ok := stats["records"].(map[string]any)
	if !ok {
		t.Fatalf("missing records section: %+v", stats)
	}
	byStatus, ok := records["byStatus"].(map[string]int)
	if !ok {
		t.Fatalf("records.byStatus wrong type: %+v", records["byStatus"])
	}
	if byStatus["captured"] < 1 {
		t.Fatalf("expected >=1 captured record, got %d", byStatus["captured"])
	}
	for _, k := range []string{"stuckOverOneHour", "enrichedLast24h", "oldestPendingSecs"} {
		if _, present := records[k]; !present {
			t.Fatalf("records section missing %q: %+v", k, records)
		}
	}
	if _, ok := stats["insights"].(map[string]any); !ok {
		t.Fatalf("missing insights section: %+v", stats)
	}
	if _, ok := stats["matches"].(map[string]any); !ok {
		t.Fatalf("missing matches section: %+v", stats)
	}
	if _, ok := stats["relationships"].(int); !ok {
		t.Fatalf("missing relationships count: %+v", stats)
	}
}
