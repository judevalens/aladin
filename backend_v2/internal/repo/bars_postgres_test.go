package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeBarSource struct{ bars []coreservice.Bar }

func (f fakeBarSource) FetchBars(context.Context, string, string, string, string) ([]coreservice.Bar, error) {
	return f.bars, nil
}

func TestBarSyncAndList(t *testing.T) {
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
	svc := coreservice.NewBarService(repo.NewBarPostgres(pool))
	base := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	src := fakeBarSource{bars: []coreservice.Bar{
		{Time: base, Open: 180, High: 185, Low: 179, Close: 184, Volume: 1_000_000},
		{Time: base.AddDate(0, 0, 1), Open: 184, High: 190, Low: 183, Close: 189, Volume: 1_200_000},
	}}
	// AAPL is seeded active by migration 00022. Idempotent: two runs both write 2.
	for i := 0; i < 2; i++ {
		n, err := svc.SyncBars(ctx, src, "AAPL", "1Day", "", "")
		if err != nil || n != 2 {
			t.Fatalf("sync run %d: n=%d err=%v", i, n, err)
		}
	}
	bars, err := svc.Get(ctx, "aapl", "1Day", 10)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(bars) != 2 || !bars[0].Time.Before(bars[1].Time) || bars[1].Close != 189 {
		t.Fatalf("expected 2 ascending bars, got %+v", bars)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM bars WHERE timeframe='1Day' AND close IN (184,189)`)
}
