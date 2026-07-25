package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/api"
	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
)

func main() {
	_ = godotenv.Load()

	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	_ = os.MkdirAll("../logs", 0o755)
	logFile, err := os.OpenFile("../logs/api.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logFile = os.Stdout
	}
	w := io.MultiWriter(os.Stdout, logFile)
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})))

	cfg, err := config.LoadAPI()
	if err != nil {
		slog.Error("api: config load failed", "component", "api", "err", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("api: db connect failed", "component", "api", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("api: migrations failed", "component", "api", "err", err)
		os.Exit(1)
	}

	deps := app.NewDependenciesWithProviderConnections(pool, cfg.ProviderConnections, cfg.DataVolumePath)
	server := api.NewWithDependencies(cfg.HTTPAddr, deps)

	// CDC outbox drain: tails outbox_events and publishes each frame to its
	// user's realtime subscribers (live delivery). Without this, the tree only
	// converges via the client's periodic/reconnect pull, never live.
	deps.OutboxDrainer().Start(ctx)

	// Live market data: hold the (single) upstream Alpaca WS and publish ticks to the outbox
	// for the drainer to fan out. No-op without Alpaca keys.
	deps.MarketData().Start(ctx)

	// Alert engine: evaluate price alerts on live ticks (via the hub's tick observer, wired in
	// wiring) + a reconcile loop that drives demand and snapshot-backstops. Started ONLY here —
	// it's a singleton tick consumer that must run in exactly one process.
	deps.AlertEngine().Start(ctx)

	go func() {
		<-ctx.Done()
		// Bounded drain: give in-flight requests up to 15s to finish, then force-close. Without a
		// deadline (the old context.Background()) a single hung handler blocks shutdown forever, so
		// the process only dies on SIGKILL. Background loops already stop on ctx cancel (safego).
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("api: shutdown failed", "component", "api", "err", err)
		}
	}()

	if err := server.Run(); err != nil {
		slog.Error("api: server stopped", "component", "api", "err", err)
		os.Exit(1)
	}
}
