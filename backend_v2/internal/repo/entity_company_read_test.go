package repo_test

import (
	"context"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEntityCompanyReadPath(t *testing.T) {
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
	r := repo.NewEntityContextPostgres(pool)
	apple := "c0000000-0000-4000-a000-0000000000a1"
	cook := "c0000000-0000-4000-a000-0000000000b1"

	dps, err := r.DataPointsFor(ctx, apple)
	if err != nil || len(dps) == 0 {
		t.Fatalf("apple data points: %v (%d)", err, len(dps))
	}
	t.Logf("apple data points: %+v", dps)

	xids, err := r.ExternalIdsFor(ctx, apple)
	if err != nil || len(xids) == 0 || xids[0].System != "cik" {
		t.Fatalf("apple external ids: %v %+v", err, xids)
	}
	t.Logf("apple external ids: %+v", xids)

	comp, err := r.CompanyFor(ctx, apple)
	if err != nil || comp == nil || comp.Sector != "Technology" || comp.Employees == 0 {
		t.Fatalf("apple company: %v %+v", err, comp)
	}
	t.Logf("apple company: %+v", comp)

	// A person has no company extension row → nil, no error.
	pc, err := r.CompanyFor(ctx, cook)
	if err != nil || pc != nil {
		t.Fatalf("person company should be nil: %v %+v", err, pc)
	}
	pdps, err := r.DataPointsFor(ctx, cook)
	if err != nil || len(pdps) == 0 {
		t.Fatalf("cook data points: %v", err)
	}
	t.Logf("cook data points: %+v", pdps)
}
