// Command backfill-bars pulls historical daily bars from Alpaca into the `bars` store for
// every active instrument (or the symbols passed as args). Idempotent — upserts by
// (instrument, timeframe, ts). Skips cleanly (exit 0) when Alpaca keys are unset.
// Run: `make ops-backfill-bars` or `go run ./cmd/backfill-bars AAPL NVDA`.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/market/alpaca"
	marketpostgres "aladin/backend_v2/internal/market/postgres"
)

// alpacaBarSource adapts the vendor client to the service's BarSource port.
type alpacaBarSource struct{ client *alpaca.Client }

func (a alpacaBarSource) FetchBars(ctx context.Context, symbol, timeframe, start, end string) ([]market.Bar, error) {
	bars, err := a.client.GetBars(ctx, symbol, timeframe, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]market.Bar, 0, len(bars))
	for _, b := range bars {
		out = append(out, market.Bar{
			Time: b.Time, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume,
		})
	}
	return out, nil
}

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("backfill-bars: DATABASE_URL is required")
		os.Exit(1)
	}
	alpacaCfg := config.LoadAlpaca()
	if !alpacaCfg.Configured() {
		slog.Warn("backfill-bars: ALPACA_API_KEY/SECRET unset — skipping")
		return
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		slog.Error("backfill-bars: db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("backfill-bars: migrate failed", "err", err)
		os.Exit(1)
	}

	// Symbols: CLI args, else every active instrument.
	symbols := os.Args[1:]
	if len(symbols) == 0 {
		rows, err := pool.Query(ctx, `SELECT symbol FROM instruments WHERE is_active ORDER BY symbol`)
		if err != nil {
			slog.Error("backfill-bars: list instruments failed", "err", err)
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
	svc := market.NewBarService(marketpostgres.NewBarPostgres(pool))
	src := alpacaBarSource{client: client}
	start := time.Now().AddDate(-2, 0, 0).Format("2006-01-02") // ~2 years of daily history

	slog.Info("backfill-bars: starting", "symbols", len(symbols), "since", start)
	var total, failures int
	for _, sym := range symbols {
		n, err := svc.SyncBars(ctx, src, sym, "1Day", start, "")
		if err != nil {
			slog.Warn("backfill-bars: sync failed", "symbol", sym, "err", err)
			failures++
			continue
		}
		total += n
	}
	slog.Info("backfill-bars: done", "bars_written", total, "failures", failures)
}
