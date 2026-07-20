// Command backfill-instruments pulls the Alpaca Assets universe into the `instruments`
// registry (T1 reference data). Idempotent — upserts by symbol, safe to re-run daily.
// Skips cleanly with exit 0 when Alpaca keys are unset (the seeded universe stays).
// Run via `make ops-backfill-instruments`.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/market/alpaca"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"
)

// alpacaAssetSource adapts the vendor client to the service's AssetSource port, mapping
// Alpaca's Asset shape onto the registry's InstrumentUpsert. Kept here (not in the alpaca
// package) so the vendor client stays free of any service dependency.
type alpacaAssetSource struct{ client *alpaca.Client }

func (a alpacaAssetSource) FetchInstruments(ctx context.Context) ([]coreservice.InstrumentUpsert, error) {
	// Active US equities only for now; delisted/survivorship handling is a later refinement.
	assets, err := a.client.ListAssets(ctx, "us_equity", "active")
	if err != nil {
		return nil, err
	}
	out := make([]coreservice.InstrumentUpsert, 0, len(assets))
	for _, as := range assets {
		if !as.Tradable {
			continue
		}
		out = append(out, coreservice.InstrumentUpsert{
			Symbol:     as.Symbol,
			Name:       as.Name,
			Exchange:   as.Exchange,
			AssetClass: as.Class,
			IsActive:   as.Status == "active",
		})
	}
	return out, nil
}

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("backfill-instruments: DATABASE_URL is required")
		os.Exit(1)
	}
	alpacaCfg := config.LoadAlpaca()
	if !alpacaCfg.Configured() {
		slog.Warn("backfill-instruments: ALPACA_API_KEY/SECRET unset — skipping (seeded universe kept)")
		return
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		slog.Error("backfill-instruments: db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("backfill-instruments: migrate failed", "err", err)
		os.Exit(1)
	}

	client := alpaca.NewClient(alpacaCfg.APIKey, alpacaCfg.APISecret, alpacaCfg.TradingBaseURL, alpacaCfg.DataBaseURL)
	svc := coreservice.NewInstrumentService(repo.NewInstrumentPostgres(pool))

	slog.Info("backfill-instruments: fetching Alpaca assets", "base", alpacaCfg.TradingBaseURL)
	n, err := svc.SyncAssets(ctx, alpacaAssetSource{client: client})
	if err != nil {
		slog.Error("backfill-instruments: sync failed", "err", err)
		os.Exit(1)
	}
	slog.Info("backfill-instruments: done", "instruments_upserted", n)
}
