// Command backfill-corporate-actions pulls splits and cash dividends from Alpaca into the
// `corporate_actions` log for every active instrument (or the symbols passed as args). This log is
// what adjust-on-read replays over the RAW bars (TRADING_PRD §5) — without it a split renders as a
// crash and any return computed across it is wrong.
//
// Idempotent — upserts by (instrument, type, ex_date), which matters more than for bars: a
// duplicated split would double-adjust all earlier history. Skips cleanly (exit 0) when Alpaca keys
// are unset. Run: `make ops-backfill-corporate-actions` or
// `go run ./cmd/backfill-corporate-actions AAPL NVDA`.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/market/alpaca"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("backfill-corporate-actions: DATABASE_URL is required")
		os.Exit(1)
	}
	alpacaCfg := config.LoadAlpaca()
	if !alpacaCfg.Configured() {
		slog.Warn("backfill-corporate-actions: ALPACA_API_KEY/SECRET unset — skipping")
		return
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		slog.Error("backfill-corporate-actions: db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("backfill-corporate-actions: migrate failed", "err", err)
		os.Exit(1)
	}

	// Symbols: CLI args, else every active instrument.
	symbols := os.Args[1:]
	if len(symbols) == 0 {
		rows, err := pool.Query(ctx, `SELECT symbol FROM instruments WHERE is_active ORDER BY symbol`)
		if err != nil {
			slog.Error("backfill-corporate-actions: list instruments failed", "err", err)
			os.Exit(1)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err == nil {
				symbols = append(symbols, s)
			}
		}
		rows.Close()
	}

	client := alpaca.NewClient(alpacaCfg.APIKey, alpacaCfg.APISecret, alpacaCfg.TradingBaseURL, alpacaCfg.DataBaseURL)
	svc := coreservice.NewBarService(repo.NewBarPostgres(pool)).
		WithCorporateActions(repo.NewCorporateActionPostgres(pool))
	src := app.NewAlpacaCorporateActionSource(client)
	// Reach back further than the bar history: an action must be known for every bar it precedes,
	// so the action window has to cover at least the bar window.
	start := time.Now().AddDate(-5, 0, 0).Format("2006-01-02")

	slog.Info("backfill-corporate-actions: starting", "symbols", len(symbols), "since", start)
	var total, failures int
	for _, sym := range symbols {
		n, err := svc.SyncCorporateActions(ctx, src, sym, start, "")
		if err != nil {
			failures++
			slog.Warn("backfill-corporate-actions: symbol failed", "symbol", sym, "err", err)
			continue
		}
		total += n
		if n > 0 {
			slog.Info("backfill-corporate-actions: synced", "symbol", sym, "actions", n)
		}
	}
	slog.Info("backfill-corporate-actions: done", "actions", total, "symbols", len(symbols), "failures", failures)
}
