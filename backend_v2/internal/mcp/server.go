package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/blocknote"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	httpServer *http.Server
	converter  blocknote.Converter
}

func New(addr string, deps app.Dependencies, converter blocknote.Converter) *Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "aladin-mcp",
		Version: "0.1.0",
	}, &sdkmcp.ServerOptions{
		Instructions: "Read-only access to Aladin pages and folders. Write tools (create_page, update_page) are temporarily disabled during the BlockNote storage migration; they return an error. Block-level write tools land in M6.",
	})
	registerTools(server, deps.Artifacts(), converter)

	streamable := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{
		JSONResponse: true,
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", bearerAuth(deps.Auth(), streamable))
	srv := &Server{converter: converter}
	mux.HandleFunc("/healthz", srv.handleHealthz)

	srv.httpServer = &http.Server{
		Addr:              addr,
		Handler:           traceRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if s.converter != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.converter.Healthz(ctx); err != nil {
			http.Error(w, "converter unreachable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) Run() error {
	slog.Info("mcp: listening", "component", "mcp", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func traceRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info(
			"mcp: request completed",
			"component", "mcp",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
