// Command backfill-graph projects the whole entity + claim layer into Neo4j (the
// connection lens): Entity/Claim nodes + ABOUT / RELATED_TO / SUPPORTS|CONTRADICTS|
// QUALIFIES / MERGED_INTO edges, then derives DIVERGES_FROM. Idempotent (MERGE) — safe to
// re-run. Requires NEO4J_URI/USER/PASS. Run via `make ops-backfill-graph`.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/graph"
	graphpostgres "aladin/backend_v2/internal/graph/postgres"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("backfill-graph: config load failed", "err", err)
		os.Exit(1)
	}
	if cfg.Neo4jURI == "" {
		slog.Error("backfill-graph: NEO4J_URI is not set — nothing to project")
		os.Exit(1)
	}
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("backfill-graph: db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	projector, err := graph.NewProjector(cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPass)
	if err != nil {
		slog.Error("backfill-graph: neo4j projector failed", "err", err)
		os.Exit(1)
	}
	defer projector.Close(ctx)
	if err := projector.Ping(ctx); err != nil {
		slog.Error("backfill-graph: neo4j unreachable", "uri", cfg.Neo4jURI, "err", err)
		os.Exit(1)
	}

	data, err := graphpostgres.NewGraphProjectionPostgres(pool).FullGraph(ctx)
	if err != nil {
		slog.Error("backfill-graph: read entity layer failed", "err", err)
		os.Exit(1)
	}
	if err := projector.Project(ctx, data); err != nil {
		slog.Error("backfill-graph: project failed", "err", err)
		os.Exit(1)
	}
	slog.Info("backfill-graph: projected entity layer into Neo4j",
		"entities", len(data.Entities),
		"merges", len(data.Merges), "related", len(data.Related))
}
