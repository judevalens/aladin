// Command backfill-entities resolves entities for every already-enriched record into
// the entity layer (R0+). Deterministic by default (no embedder/adjudicator) — safe and
// free to re-run, and idempotent (aliases/mentions are upserts). Run via
// `make ops-backfill-entities`.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("backfill-entities: config load failed", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("backfill-entities: db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("backfill-entities: migrate failed", "err", err)
		os.Exit(1)
	}

	recordRepo := db.NewRecordRepository(pool)
	resolver := entities.NewResolver(db.NewEntityRepository(pool))

	rows, err := pool.Query(ctx, `SELECT id FROM records WHERE status = 'enriched' ORDER BY created_at`)
	if err != nil {
		slog.Error("backfill-entities: list records failed", "err", err)
		os.Exit(1)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			slog.Error("backfill-entities: scan failed", "err", err)
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	rows.Close()

	slog.Info("backfill-entities: starting", "records", len(ids))
	var records, mentions, failures int
	for _, id := range ids {
		rec, err := recordRepo.Get(ctx, id)
		if err != nil {
			slog.Warn("backfill-entities: get record failed", "record_id", id, "err", err)
			failures++
			continue
		}
		for _, surface := range rec.Enrichment.Entities {
			if _, err := resolver.Resolve(ctx, entities.Mention{
				Surface:        surface,
				RecordID:       rec.ID,
				SourceRevision: rec.SourceRevision,
				ContextHint:    rec.Enrichment.Summary,
			}); err != nil {
				slog.Warn("backfill-entities: resolve failed", "record_id", id, "surface", surface, "err", err)
				failures++
				continue
			}
			mentions++
		}
		records++
		if records%100 == 0 {
			slog.Info("backfill-entities: progress", "records", records, "mentions", mentions)
		}
	}
	slog.Info("backfill-entities: done", "records", records, "mentions", mentions, "failures", failures)
}
