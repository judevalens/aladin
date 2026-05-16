package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	mcpserver "aladin/backend_v2/internal/mcp"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	_ = os.MkdirAll("../logs", 0o755)
	logFile, err := os.OpenFile("../logs/mcp.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logFile = os.Stdout
	}
	w := io.MultiWriter(os.Stdout, logFile)
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})))

	cfg, err := config.LoadMCP()
	if err != nil {
		slog.Error("mcp: config load failed", "component", "mcp", "err", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("mcp: db connect failed", "component", "mcp", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("mcp: migrations failed", "component", "mcp", "err", err)
		os.Exit(1)
	}

	server := mcpserver.New(cfg.HTTPAddr, app.NewDependencies(pool))

	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("mcp: shutdown failed", "component", "mcp", "err", err)
		}
	}()

	if err := server.Run(); err != nil {
		slog.Error("mcp: server stopped", "component", "mcp", "err", err)
		os.Exit(1)
	}
}
