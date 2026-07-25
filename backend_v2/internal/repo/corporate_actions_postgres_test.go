package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCorporateActions_RoundTripAndAdjustedRead exercises migration 00034 + the repo + the
// end-to-end adjust-on-read path: raw bars stay raw in the DB, while the service replays the
// action log so a 4-for-1 split reads as price continuity instead of a -75% crash.
func TestCorporateActions_RoundTripAndAdjustedRead(t *testing.T) {
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

	instrumentID := uuid.NewString()
	symbol := "CA" + uuid.NewString()[:6] // unique: instruments has a unique active-symbol constraint
	if _, err := pool.Exec(ctx,
		`INSERT INTO instruments (instrument_id, symbol, name) VALUES ($1::uuid,$2,'CorpAction Co')`,
		instrumentID, symbol); err != nil {
		t.Fatalf("seed instrument: %v", err)
	}
	t.Cleanup(func() {
		// corporate_actions + bars cascade on the instrument FK.
		_, _ = pool.Exec(context.Background(), `DELETE FROM instruments WHERE instrument_id = $1::uuid`, instrumentID)
	})

	// Raw bars: 400 pre-split, 100 post-split.
	day := func(d int) time.Time { return time.Date(2024, 6, d, 0, 0, 0, 0, time.UTC) }
	barRepo := repo.NewBarPostgres(pool)
	if _, err := barRepo.UpsertBars(ctx, []coreservice.BarUpsert{
		{Symbol: symbol, Timeframe: "1Day", Bar: coreservice.Bar{Time: day(1), Open: 400, High: 400, Low: 400, Close: 400, Volume: 100}},
		{Symbol: symbol, Timeframe: "1Day", Bar: coreservice.Bar{Time: day(3), Open: 100, High: 100, Low: 100, Close: 100, Volume: 400}},
	}); err != nil {
		t.Fatalf("upsert bars: %v", err)
	}

	caRepo := repo.NewCorporateActionPostgres(pool)
	actions := []coreservice.CorporateAction{
		{Type: coreservice.ActionSplit, ExDate: day(3), SplitRatio: 4},
		{Type: coreservice.ActionCashDividend, ExDate: day(2), CashAmount: 1},
	}
	n, err := caRepo.UpsertActions(ctx, symbol, actions)
	if err != nil {
		t.Fatalf("upsert actions: %v", err)
	}
	if n != 2 {
		t.Fatalf("wrote %d actions, want 2", n)
	}
	// Idempotent: a re-sync must not duplicate (a duplicated split would double-adjust history).
	if _, err := caRepo.UpsertActions(ctx, symbol, actions); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err := caRepo.ListActions(ctx, symbol)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("after re-sync there are %d actions, want 2 (upsert must be idempotent)", len(got))
	}
	if !got[0].ExDate.Before(got[1].ExDate) {
		t.Fatalf("actions must come back oldest-first, got %v then %v", got[0].ExDate, got[1].ExDate)
	}

	svc := coreservice.NewBarService(barRepo).WithCorporateActions(caRepo)

	// Raw storage is untouched.
	raw, err := svc.GetAdjusted(ctx, symbol, "1Day", 10, coreservice.AdjustNone)
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	if len(raw) != 2 || raw[0].Close != 400 {
		t.Fatalf("raw read = %+v, want the stored 400 (bars must be stored unadjusted)", raw)
	}

	// Split-adjusted: the pre-split bar divides by 4 → continuity with the post-split bar.
	split, err := svc.GetAdjusted(ctx, symbol, "1Day", 10, coreservice.AdjustSplits)
	if err != nil {
		t.Fatalf("get split-adjusted: %v", err)
	}
	if split[0].Close != 100 {
		t.Fatalf("split-adjusted close = %v, want 100", split[0].Close)
	}
	if split[0].Volume != 400 {
		t.Fatalf("split-adjusted volume = %d, want 400 (×4)", split[0].Volume)
	}

	// Total-return also replays the $1 dividend: 400 × (400-1)/400 ÷ 4.
	total, err := svc.GetAdjusted(ctx, symbol, "1Day", 10, coreservice.AdjustTotal)
	if err != nil {
		t.Fatalf("get total-adjusted: %v", err)
	}
	want := 400 * (400 - 1.0) / 400 / 4
	if diff := total[0].Close - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("total-return close = %v, want %v", total[0].Close, want)
	}
}
