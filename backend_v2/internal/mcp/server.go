package mcpserver

import (
	"aladin/backend_v2/internal/alert"
	"context"
	"log/slog"
	"net/http"
	"time"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/search"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/watchlist"

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

Folder workflow:
  - get_browser_tree or list_folders to inspect current structure
  - create_folder(title, parent_id?) to add a root or nested folder
  - rename_folder(id, title) to rename a folder
  - move_artifact(artifact_id, folder_id?) to move a page/app/file/link/voice;
    omit folder_id or pass null to move it to the root

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

Shard capabilities are discovered, not assumed: call get_authoring_guide before
answering what can be built. Without page_id it describes new shards on this
backend; with page_id it describes the supported APIs for that specific shard.
Use those enabled capabilities directly; do not ask the user to choose a runtime
version or infer a shard data source from unrelated workspace/market tools.

Authoring loop:
  1. For a NEW shard, create_app seeds the files/data configuration enabled here
     and returns authoring_guide, current_index_tsx and any data contract.
     For an EXISTING shard, call get_authoring_guide(page_id), then read its files.
     Preserve its saved data and storage API; do not migrate during a routine edit.
  2. Build the UI with ordinary React and semantic HTML. Use Aladin's token-backed
     Tailwind utilities by default; import nonvisual APIs from @aladin/shard.
     Keep data-anchor attributes and their anchors.json declarations in sync.
     Entry point: index.tsx, mounting #root via createRoot from react-dom/client.
  3. write_file creates files; overwrite:true is required to replace an existing
     file. Prefer edit_file for targeted changes. Previous bytes go to .history/.
     Writes auto-build draft and return diagnostics; fix errors until build.ok.
     For bulk writes use build:false, then build once.
  4. install_lib(page_id,name) installs compatible dependencies through esm.sh,
     e.g. recharts or d3. Import them normally; never load remote runtime scripts.
  5. build_app compiles publishable code. preview_open, preview_navigate,
     preview_snapshot/screenshot, preview_click and preview_console exercise it.
     Preview execution can write draft data; never treat emulator values as user
     data. Use the target's guide for exact persistence/observation guarantees.
  6. verify_app checks anchors, source declarations, routes and rendering.
     Fix reported failures. publish_app performs the target's required checks
     and makes the verified artifact live. Renderer requirements come from the
     current guide; a successful build alone is not evidence of correct rendering.

Use hash routing, never pushState or BrowserRouter. Every internal link must
retain the document URL and credential
through a fragment (#/section); relative navigation breaks authentication and is
rejected by verification. Use injected CSS variables such as var(--color-ink)
and var(--color-for) for chart/SVG colors rather than hardcoded palette values.

The iframe is isolated from Aladin's DOM/session. Workspace access uses the
host-authorized API described by the guide. External HTTPS/WSS requests are
subject to browser/CORS restrictions; they confer no workspace permissions and
must not contain private credentials. Remote code uses the build/vendor pipeline.

Workspace tools — ground answers in the user's Aladin data:
  - search(query) FIRST to find ids: tickers, entities (companies/people),
    pages, shards. Then get_entity / get_artifact / get_page to read one.
  - get_insights lists engine-generated insights; get_watchlist / get_bars /
    get_quote cover the Markets surface; create_alert (recurring, self-re-arming
    price alert; surfaces as a notification) / list_alerts / delete_alert;
    watchlists are NAMED instrument sets (a user keeps several) — get_watchlist /
    list_watchlists / create_watchlist, and add_to_watchlist takes an optional
    list name (created if new). add_to_watchlist and draw_edge are
    light additive writes.
  - Market intelligence: get_news (catalysts — use it to explain WHY a stock
    moved, not just that it did), get_movers + get_most_actives (what's moving /
    where liquidity is, no symbol needed), and read-only account state:
    get_account (cash/equity/buying power) + get_positions (actual holdings +
    unrealized P&L — reason about REAL exposure, not the abstract watchlist).
    These cannot place or modify orders. When get_account.paper is true, say the
    numbers are from a paper (simulated) account.
    There is no VIX/index feed: use ETF proxies — SPY (S&P 500), QQQ (Nasdaq
    100), IWM (Russell 2000), DIA (Dow), VIXY (volatility), and sector SPDRs
    (XLK tech, XLF financials, XLE energy, XLV health, XLY discretionary,
    XLP staples, XLI industrials, XLU utilities, XLB materials, XLRE real
    estate, XLC comms). There is no earnings-calendar/fundamentals tool — say so
    plainly rather than guessing dates or EPS.
  - Citations: when a tool result carries a "citations" array ({kind,id,title}),
    those items ground the answer — prefer tools that return them, and rely on
    cited material over memory when answering about the user's data.`

type Server struct {
	httpServer *http.Server
	converter  blocknote.Converter
	preview    service.PreviewService
}

// Dependencies is the MCP consumer's complete service contract. It is defined
// here so adding an API/worker-only dependency cannot silently widen MCP.
type Dependencies interface {
	Auth() service.AuthService
	Artifacts() service.ArtifactService
	Insights() insights.InsightService
	DocSurfaceStore() service.DocSurfaceStore
	Preview() service.PreviewService
	ShardBuild() service.ShardBuildService
	ShardResources() shardresource.Service
	ShardGraphQL() service.ShardGraphQLService
	ShardReleases() service.ShardReleaseService
	ShardCatalog() service.ShardCatalogService
	ShardBridge() service.ShardBridgeService
	Documents() service.DocumentService
	EntityTags() service.EntityTagService
	ArtifactRefs() service.ArtifactRefService
	EntityContext() service.EntityContextService
	Instruments() instrument.InstrumentService
	Watchlist() watchlist.Service
	Search() search.SearchService
	Bars() market.BarService
	QuoteSnapshots() market.QuoteSnapshotSource
	MarketInfo() market.MarketInfoService
	Alerts() alert.AlertService
}

func New(addr string, deps Dependencies, pages service.PageDocumentService, converter blocknote.Converter, bridge blocknote.Bridge) *Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "aladin-mcp",
		Version: "0.1.0",
	}, &sdkmcp.ServerOptions{
		Instructions: mcpInstructions,
	})
	registerTools(server, deps.Artifacts(), pages, converter, bridge, deps.EntityTags(), deps.ArtifactRefs())
	registerShardResourceTools(server, deps.ShardResources(), deps.ShardCatalog(), deps.ShardGraphQL())
	registerDocSurfaceTools(server, deps.Artifacts(), deps.DocSurfaceStore(), deps.ShardBuild(), deps.Preview(), deps.ShardBridge(), deps.ShardReleases(), deps.ShardGraphQL())
	registerWorkspaceTools(server, workspaceToolServer{
		search:      deps.Search(),
		entities:    deps.EntityContext(),
		insights:    deps.Insights(),
		artifacts:   deps.Artifacts(),
		watchlist:   deps.Watchlist(),
		bars:        deps.Bars(),
		snapshots:   deps.QuoteSnapshots(),
		marketInfo:  deps.MarketInfo(),
		alerts:      deps.Alerts(),
		instruments: deps.Instruments(),
		documents:   deps.Documents(),
	})

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

// statusRecorder captures the response code so the access log can report it.
// http.ResponseWriter has no getter, and WriteHeader may never be called at all
// (an implicit 200 on first Write), so default to 200 and record explicitly.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush keeps the MCP streamable-HTTP GET working: that response is a long-lived
// SSE stream, and wrapping the writer would otherwise hide http.Flusher from it.
func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer (deadlines, flush).
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func traceRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		// Status matters here: a 401 and a healthy handshake were previously
		// indistinguishable in this log, which made "MCP unreachable" from the
		// copilot impossible to diagnose from the server side.
		slog.Info(
			"mcp: request completed",
			"component", "mcp",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
