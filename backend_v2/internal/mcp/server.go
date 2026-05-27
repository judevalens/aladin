package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpInstructions teaches an LLM client (Claude Code, Codex, etc.) how to
// drive the Aladin MCP surface. Kept as a constant so it shows up clearly
// in code review when the contract changes.
const mcpInstructions = `Aladin pages are ordered lists of blocks. Each block has:
  - a stable id (string)
  - a type (paragraph, heading, bulletListItem, codeBlock, etc.)
  - a markdown rendering you can read and edit

You always work in markdown. The server handles conversion to and from
BlockNote's internal block format.

Editing workflow:
  1. get_page(id)            → see the block list with ids and per-block markdown
  2. update_block(id, ...)   → replace one block's content
  3. insert_blocks(...)      → add new blocks at a position relative to an id
  4. delete_block(id)        → remove a block
  5. update_page(id, ...)    → only when rewriting the whole document

Prefer block-level operations for surgical edits. update_page wipes ids,
breaks downstream references, and triggers a full re-index — use sparingly.

Notes:
  - update_block accepts markdown that may parse into multiple blocks (e.g.
    a bullet list with three items becomes three blocks). The original id
    is kept on the first produced block.
  - Lists are individual blocks per item, not one block per list.
  - When unsure of structure, get_page first.`

type Server struct {
	httpServer *http.Server
	converter  blocknote.Converter
}

func New(addr string, deps app.Dependencies, pages service.PageDocumentService, converter blocknote.Converter) *Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "aladin-mcp",
		Version: "0.1.0",
	}, &sdkmcp.ServerOptions{
		Instructions: mcpInstructions,
	})
	registerTools(server, deps.Artifacts(), pages, converter)

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
