package docsurface

// Interactive preview sessions for Doc Surface ("app" artifacts).
//
// A PreviewSessions manager keeps a long-lived headless Chrome tab alive per
// (userID, pageID) across MCP calls, so an agent can OPEN a built page and then
// interactively NAVIGATE its in-app hash routes, SNAPSHOT the DOM, SCREENSHOT,
// EVAL JS, CLICK elements, and read the accumulated CONSOLE — before publishing.
//
// Security: the tab loads the exact same inlined bundle (PreviewHTML) under the
// exact same opaque-origin CSP as the served iframe. eval/click therefore run
// against untrusted agent code with zero host reach — the same boundary as
// production. The browser is only ever launched lazily; where no Chrome binary
// exists, every method returns a clean "renderer unavailable" BadRequest and the
// rest of the MCP surface keeps working.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/service"

	page "github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

const (
	viewportW = 1024
	viewportH = 768

	// openTimeout bounds the load (about:blank → set content → mount wait).
	openTimeout = 30 * time.Second
	// opTimeout bounds a single interactive op (navigate/snapshot/eval/click).
	opTimeout = 15 * time.Second
	// mountTimeout is how long we poll for React to render into #root before
	// reporting Mounted=false (best-effort — never an error).
	mountTimeout = 3 * time.Second
	// settleDelay lets a hashchange re-render / a click handler flush.
	settleDelay = 150 * time.Millisecond

	maxLogLines    = 500 // ring-buffer cap per accumulator
	logReturn      = 100 // how many recent lines each op returns
	reaperInterval = time.Minute

	defaultIdleTTL     = 10 * time.Minute
	defaultMaxSessions = 16
)

// PreviewOptions tunes the session manager. Zero values fall back to defaults.
type PreviewOptions struct {
	IdleTTL     time.Duration
	MaxSessions int
	// Builder, when set, is used for the rebuild every Open performs, so a
	// preview build records shard_build_state and emits a build-status event
	// like any other build. Without it Open falls back to the raw runtime and
	// the work pane's status chip goes stale while an agent iterates.
	Builder   service.ShardBuildService
	Resources service.ShardResourceService
	Releases  service.ShardReleaseService
	GraphQL   service.ShardGraphQLService
}

// PreviewSessions is the service.PreviewService impl. One manager owns a shared
// browser (ExecAllocator) and a tab per (userID, pageID).
type PreviewSessions struct {
	store     service.DocSurfaceStore
	runtime   service.WorkspaceRuntime
	builder   service.ShardBuildService // optional; see PreviewOptions.Builder
	resources service.ShardResourceService
	releases  service.ShardReleaseService
	graphql   service.ShardGraphQLService

	idleTTL      time.Duration
	maxSessions  int
	tickInterval time.Duration // reaper poll cadence (≤ idleTTL so small TTLs reap promptly)

	mu          sync.Mutex
	sessions    map[string]*previewSession
	allocCtx    context.Context
	allocCancel context.CancelFunc
	allocReady  bool
	initErr     error // cached "renderer unavailable" (no Chrome binary)

	reaperStop     chan struct{}
	stopReaperOnce sync.Once

	// Local static server over the vendored deps, so the about:blank preview can
	// load them via the import map. Started lazily, guarded by mu.
	vendorStarted bool
	vendorBase    string
	vendorSrv     *http.Server
	vendorErr     error
}

// previewSession is one live tab. opMu serializes chromedp ops on the tab; logMu
// guards the console/exception accumulators written from the ListenTarget
// callback. lastUsed is guarded by the manager mutex.
type previewSession struct {
	key       string
	tabCtx    context.Context
	tabCancel context.CancelFunc

	opMu    sync.Mutex
	started bool // browser/tab allocated (first Run done); guarded by opMu

	logMu         sync.Mutex
	console       []string
	consoleErrors []string
	exceptions    []string

	lastUsed       time.Time
	resourceMu     sync.Mutex
	resourceQueue  chan string
	resourceCancel context.CancelFunc
}

// NewPreviewSessions constructs the manager and starts the idle reaper. It does
// NOT touch Chrome — the browser is launched lazily on the first Open.
func NewPreviewSessions(store service.DocSurfaceStore, runtime service.WorkspaceRuntime, opts PreviewOptions) service.PreviewService {
	if opts.IdleTTL <= 0 {
		opts.IdleTTL = defaultIdleTTL
	}
	if opts.MaxSessions <= 0 {
		opts.MaxSessions = defaultMaxSessions
	}
	tick := opts.IdleTTL
	if tick > reaperInterval {
		tick = reaperInterval
	}
	if tick < 50*time.Millisecond {
		tick = 50 * time.Millisecond
	}
	m := &PreviewSessions{
		store:        store,
		runtime:      runtime,
		builder:      opts.Builder,
		resources:    opts.Resources,
		releases:     opts.Releases,
		graphql:      opts.GraphQL,
		idleTTL:      opts.IdleTTL,
		maxSessions:  opts.MaxSessions,
		tickInterval: tick,
		sessions:     map[string]*previewSession{},
		reaperStop:   make(chan struct{}),
	}
	go m.reaper()
	return m
}

// --- public API (service.PreviewService) -----------------------------------

func (m *PreviewSessions) Open(ctx context.Context, pageID string, channel service.BuildChannel, opts service.PreviewOpenOptions) (service.PreviewState, error) {
	key, err := sessionKey(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	// Rebuild on every open so the tab always reflects the agent's latest edits.
	// Through the build SERVICE when available, so the status the work pane
	// shows tracks what the agent is previewing.
	var res service.BuildResult
	if opts.Build != nil {
		res = *opts.Build
	} else {
		res, err = m.build(ctx, pageID, channel)
	}
	if err != nil {
		return service.PreviewState{}, err
	}
	if !res.OK {
		return service.PreviewState{}, service.BadRequest("build failed; fix the errors and preview_open again:\n" + res.Log)
	}
	theme := opts.Theme
	if !ValidTheme(theme) {
		theme = ""
	}
	distRel := DistDir(channel)
	var bundleJS, bundleCSS []byte
	if len(res.Contract) > 0 {
		if m.resources == nil || m.releases == nil {
			return service.PreviewState{}, service.BadRequest("V2 preview resources unavailable")
		}
		// Preview is always a real draft sandbox, including verification of a
		// staged published build. It can never write published records.
		if err := m.releases.Stage(ctx, pageID, service.ChannelDraft, res); err != nil {
			return service.PreviewState{}, err
		}
		if err := m.releases.Activate(ctx, pageID, service.ChannelDraft, res.BuildID); err != nil {
			return service.PreviewState{}, err
		}
		bundleJS, bundleCSS = res.Files["bundle.js"], res.Files["bundle.css"]
	} else {
		bundleJS, err = m.store.ReadFile(ctx, pageID, distRel+"/bundle.js")
		bundleCSS, _ = m.store.ReadFile(ctx, pageID, distRel+"/bundle.css")
	}
	if err != nil {
		return service.PreviewState{}, err
	}
	html, err := m.previewHTML(ctx, pageID, distRel, string(bundleCSS), string(bundleJS), theme, &res)
	if err != nil {
		return service.PreviewState{}, err
	}

	// Load into a tab. If the browser/tab died under us (crash, external kill),
	nonce := uuid.NewString()
	html = strings.ReplaceAll(html, "__PREVIEW_RESOURCE_NONCE__", nonce)
	// self-heal once: reset the dead browser and retry on a fresh one. Combined
	// with the liveness checks in ensureAllocLocked/getOrCreate, this means a
	// crashed renderer recovers on the next preview_open — no mcp restart.
	for attempt := 0; ; attempt++ {
		st, err := m.openOnce(key, pageID, html, func(s *previewSession) error { return m.configureResourcePreview(ctx, s, pageID, res, nonce) })
		if err == nil {
			return st, nil
		}
		if attempt == 0 && isBrowserDead(err) {
			m.resetBrowser()
			continue
		}
		return service.PreviewState{}, err
	}
}

// build runs the page's build through the status-recording service when one is
// wired, else straight through the runtime.
func (m *PreviewSessions) build(ctx context.Context, pageID string, channel service.BuildChannel) (service.BuildResult, error) {
	if m.builder != nil {
		return m.builder.Build(ctx, pageID, channel)
	}
	return m.runtime.Build(ctx, pageID, channel)
}

// openOnce performs one load attempt: (re)acquire the tab, warm up the browser
// on first use, then replace the document and wait for mount.
func (m *PreviewSessions) openOnce(key, pageID, html string, setup ...func(*previewSession) error) (service.PreviewState, error) {
	s, err := m.getOrCreate(key)
	if err != nil {
		return service.PreviewState{}, err
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.resetLogs()

	// The FIRST Run on a tab allocates the browser; per chromedp, a timeout on
	// that first Run would tear the whole browser down — so warm up on the raw
	// tab context (watchdog-bounded), enabling the domains + establishing a
	// top-level frame. Subsequent ops use timeout children safely.
	if !s.started {
		if err := runFirst(s, openTimeout,
			cdpruntime.Enable(),
			page.Enable(),
			chromedp.EmulateViewport(viewportW, viewportH),
			chromedp.Navigate("about:blank"),
		); err != nil {
			return service.PreviewState{}, fmt.Errorf("start browser: %w", err)
		}
		s.started = true
	}

	opCtx, cancel := context.WithTimeout(s.tabCtx, openTimeout)
	defer cancel()
	if len(setup) > 0 {
		if err := setup[0](s); err != nil {
			return service.PreviewState{}, err
		}
	}
	if err := chromedp.Run(opCtx,
		setDocContent(html),
		waitMount(mountTimeout),
	); err != nil {
		return service.PreviewState{}, fmt.Errorf("load preview: %w", err)
	}
	return m.captureState(opCtx, s, pageID), nil
}

// previewHTML builds the preview document. For an ESM build (importmap.json
// present), it absolutizes the /vendor URLs to the local vendor server and widens
// the meta-CSP to that origin; otherwise it serves the legacy inline doc.
func (m *PreviewSessions) previewHTML(ctx context.Context, pageID, distRel, css, js, theme string, builds ...*service.BuildResult) (string, error) {
	var im ImportMap
	csp := CSP
	var build *service.BuildResult
	if len(builds) > 0 && len(builds[0].Contract) > 0 {
		build = builds[0]
	}
	data, derr := m.store.ReadFile(ctx, pageID, distRel+"/importmap.json")
	if build != nil {
		data, derr = build.Files["importmap.json"], nil
	}
	if derr == nil {
		if json.Unmarshal(data, &im) == nil {
			if im.Imports == nil {
				im.Imports = map[string]string{}
			}
			if len(im.Imports) > 0 {
				base, err := m.ensureVendorServer()
				if err != nil {
					return "", fmt.Errorf("preview vendor server: %w", err)
				}
				abs := make(map[string]string, len(im.Imports))
				for spec, u := range im.Imports {
					abs[spec] = base + u // u is "/vendor/<sha>"
				}
				im.Imports = abs
				csp = CSPWithVendor(base)
			}
		}
	}
	if build != nil {
		csp = CSPForBridgeVersion(csp, "bridge/2")
		html := PreviewHTML(pageID, TokensCSS, css, js, csp, im, theme)
		html = strings.Replace(html, breakInlineClosers(previewBridgeEmulatorJS), previewResourceBridgeJS, 1)
		return BootstrapV2(html, previewResourceRelease(*build)), nil
	}
	return PreviewHTML(pageID, TokensCSS, css, js, csp, im, theme), nil
}

// ensureVendorServer lazily starts a localhost static server over the vendored
// deps so the about:blank preview can load them via the import map. Production
// serves /vendor from the API origin; this is preview-only.
func (m *PreviewSessions) ensureVendorServer() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vendorStarted {
		return m.vendorBase, m.vendorErr
	}
	m.vendorStarted = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		m.vendorErr = err
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /vendor/{sha}", func(w http.ResponseWriter, r *http.Request) {
		data, err := m.runtime.ReadVendor(r.PathValue("sha"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(data)
	})
	m.vendorSrv = &http.Server{Handler: mux}
	m.vendorBase = "http://" + ln.Addr().String()
	go func() { _ = m.vendorSrv.Serve(ln) }()
	return m.vendorBase, nil
}

// --- session lifecycle -----------------------------------------------------
