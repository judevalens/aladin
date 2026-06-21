// Command backfill-signals re-emits durable signal frames (entity kind "signal") for every record
// that has produced claims, to that record's subscribers — so an already-synced client receives the
// claims that existed before the sync path was wired. Idempotent (bumps seq; the client's seq guard
// applies the latest). Run via `make ops-backfill-signals`.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("backfill-signals: config load failed", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("backfill-signals: db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	n, err := repo.NewSignalSyncRepo(pool).BackfillAll(ctx)
	if err != nil {
		slog.Error("backfill-signals: backfill failed", "err", err)
		os.Exit(1)
	}
	slog.Info("backfill-signals: re-emitted signal frames to subscribers", "records", n)
}
