package repo_test

import (
	"context"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInstrumentSearchSmoke(t *testing.T) {
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
	r := repo.NewInstrumentPostgres(pool)
	for _, q := range []string{"NVDA", "nvid", "GOOG", "apple"} {
		hits, err := r.SearchInstruments(ctx, q, 10)
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if len(hits) == 0 {
			t.Fatalf("search %q returned no hits", q)
		}
		t.Logf("q=%q -> top=%s (%s) [%d hits]", q, hits[0].Symbol, hits[0].Name, len(hits))
	}
}

type fakeAssetSource struct {
	rows []coreservice.InstrumentUpsert
}

func (f fakeAssetSource) FetchInstruments(context.Context) ([]coreservice.InstrumentUpsert, error) {
	return f.rows, nil
}

func TestInstrumentSyncAssetsUpsertsAndIsSearchable(t *testing.T) {
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
	svc := coreservice.NewInstrumentService(repo.NewInstrumentPostgres(pool))
	src := fakeAssetSource{rows: []coreservice.InstrumentUpsert{
		{Symbol: "ZTST", Name: "Zeta Test Corp", Exchange: "NYSE", AssetClass: "us_equity", IsActive: true},
	}}
	// Idempotent: two runs both succeed; second is an upsert no-op on the row.
	for i := 0; i < 2; i++ {
		n, err := svc.SyncAssets(ctx, src)
		if err != nil {
			t.Fatalf("sync run %d: %v", i, err)
		}
		if n != 1 {
			t.Fatalf("sync run %d wrote %d, want 1", i, n)
		}
	}
	hits, err := svc.Search(ctx, "ZTST", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Name != "Zeta Test Corp" {
		t.Fatalf("expected synced instrument searchable, got %+v", hits)
	}
	// Cleanup so the sandbox row doesn't leak into other tests.
	_, _ = pool.Exec(ctx, `DELETE FROM instruments WHERE symbol = 'ZTST'`)
}
