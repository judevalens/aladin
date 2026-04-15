package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"aladin/backend_v2/internal/api"
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

	cfg := config.LoadAPI()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool := db.Connect(ctx, cfg.DatabaseURL)
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("api: migrations failed", "component", "api", "err", err)
		os.Exit(1)
	}

	server := api.New(cfg.HTTPAddr, pool)

	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("api: shutdown failed", "component", "api", "err", err)
		}
	}()

	if err := server.Run(); err != nil {
		slog.Error("api: server stopped", "component", "api", "err", err)
		os.Exit(1)
	}
}
