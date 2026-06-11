package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/blocknote"
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

	converter := blocknote.NewClient(cfg.ConverterURL, blocknote.ClientOptions{
		AdminSecret: cfg.ConverterAdminSecret,
	})
	if err := waitForConverter(ctx, converter); err != nil {
		slog.Warn("mcp: converter not reachable at startup, continuing anyway", "component", "mcp", "url", cfg.ConverterURL, "err", err)
	} else {
		slog.Info("mcp: converter reachable", "component", "mcp", "url", cfg.ConverterURL)
	}

	deps := app.NewDependenciesWithProviderConnections(pool, config.LoadProviderConnections(), cfg.DataVolumePath)
	// One *Client serves both jobs: markdown<->blocks conversion and the
	// /admin/* collab bridge (same base URL, shared secret).
	server := mcpserver.New(cfg.HTTPAddr, deps, deps.PageDocuments(), converter, converter)

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

// waitForConverter performs a short retry loop against /healthz so the MCP
// server doesn't return 500s during the first second of a cold
// `docker-compose up`. We tolerate a missing converter at startup because
// every tool call that needs it will surface a clear error of its own; the
// retry just smooths the common case of the converter coming up alongside.
func waitForConverter(ctx context.Context, c *blocknote.Client) error {
	const attempts = 5
	var lastErr error
	for i := 0; i < attempts; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := c.Healthz(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
