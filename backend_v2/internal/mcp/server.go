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
  - When unsure of structure, get_page first.

Doc Surface ("app" artifacts) — a SEPARATE surface from pages. An app is an
interactive React app you author as files, build, and publish; it renders in a
sandboxed iframe. Use this (not create_page) when the user wants an interactive
widget, dashboard, visualization, calculator, or mini-tool.

Authoring loop (like a normal dev):
  1. create_app(title)        → makes the page + seeds index.tsx; returns its id
  2. write_file(page_id, path, content) → author index.tsx + any components/css.
     Write standard React/TS: import { useState } from "react"; mount onto
     #root via createRoot from "react-dom/client". JSX is automatic (no React
     import needed for JSX itself).
  3. install_lib(page_id, name) → add a dependency (e.g. "recharts", "d3");
     import it normally. It is bundled from esm.sh at build time.
  4. build_app(page_id)        → compiles. On {ok:false}, read log, fix, rebuild.
  5. preview_open(page_id)      → build + open a LIVE headless tab. Verify it
     actually RUNS (compiling != working): expect mounted:true and an empty
     exceptions list. Walk every route and inspect before publishing.
  6. publish_app(page_id, summary) → records a markdown summary (the knowledge-
     graph spine — describe what the app shows) and marks it live. Requires a
     successful build_app first.

Multi-page docs — ONE app, HASH routing:
  A Doc is a single React app. "Pages"/nested sections are CLIENT-SIDE HASH
  routes (#/, #/section, #/section/sub) navigated in-place — never reload, and
  never use pushState/history routing (the page is served at an opaque origin and
  would break on reload). Lightest pattern, no dependency:

    import { useState, useEffect } from "react";
    function useRoute() {
      const [h, setH] = useState(location.hash || "#/");
      useEffect(() => {
        const on = () => setH(location.hash || "#/");
        addEventListener("hashchange", on);
        return () => removeEventListener("hashchange", on);
      }, []);
      return h;
    }
    // render: const route = useRoute(); switch on route; links are <a href="#/sub">.
    // Wrap each view in <div id="view-root" data-route={route}> so navigation settles.

  For richer nested routing, install_lib("react-router-dom") and use HashRouter
  (NOT BrowserRouter).

Inspecting (do this before publish):
  - preview_open → confirm mounted:true and exceptions empty.
  - preview_navigate(page_id, "#/route") → go to each route, then preview_snapshot
    / preview_screenshot to confirm the right view rendered, preview_eval to read
    app state, preview_click(selector) to exercise flows (menus, in-app links).
  - preview_console → confirm no uncaught exceptions accumulated.
  - Found a bug? write_file the fix, then preview_open again (it rebuilds + reloads
    the same tab). Only publish_app once EVERY route mounts clean.
  - "renderer unavailable" means no headless browser is present — that is NOT a
    build failure; fall back to build_app + reading the log.

Notes:
  - read_file / list_dir to inspect; write_file overwrites whole files.
  - The runtime is isolated: no network at runtime, no access to Aladin. Keep all
    state in-app. Entry point is always index.tsx at the page root.`

type Server struct {
	httpServer *http.Server
	converter  blocknote.Converter
	preview    service.PreviewService
}

func New(addr string, deps app.Dependencies, pages service.PageDocumentService, converter blocknote.Converter, bridge blocknote.Bridge) *Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "aladin-mcp",
		Version: "0.1.0",
	}, &sdkmcp.ServerOptions{
		Instructions: mcpInstructions,
	})
	registerTools(server, deps.Artifacts(), pages, converter, bridge)
	registerDocSurfaceTools(server, deps.Artifacts(), deps.DocSurfaceStore(), deps.WorkspaceRuntime(), deps.Preview())

	streamable := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{
		JSONResponse: true,
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", bearerAuth(deps.Auth(), streamable))
	srv := &Server{converter: converter, preview: deps.Preview()}
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
	// Close live preview tabs + the headless browser before we stop serving.
	if s.preview != nil {
		_ = s.preview.CloseAll(ctx)
	}
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
