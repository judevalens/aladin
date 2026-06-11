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
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/service"

	page "github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
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
}

// PreviewSessions is the service.PreviewService impl. One manager owns a shared
// browser (ExecAllocator) and a tab per (userID, pageID).
type PreviewSessions struct {
	store   service.DocSurfaceStore
	runtime service.WorkspaceRuntime

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

	logMu      sync.Mutex
	console    []string
	exceptions []string

	lastUsed time.Time
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

func (m *PreviewSessions) Open(ctx context.Context, pageID string) (service.PreviewState, error) {
	key, err := sessionKey(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	// Rebuild on every open so the tab always reflects the agent's latest edits.
	res, err := m.runtime.Build(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	if !res.OK {
		return service.PreviewState{}, service.BadRequest("build failed; fix the errors and preview_open again:\n" + res.Log)
	}
	bundleJS, err := m.store.ReadFile(ctx, pageID, distDirName+"/bundle.js")
	if err != nil {
		return service.PreviewState{}, err
	}
	bundleCSS, _ := m.store.ReadFile(ctx, pageID, distDirName+"/bundle.css")
	html, err := m.previewHTML(ctx, pageID, string(bundleCSS), string(bundleJS))
	if err != nil {
		return service.PreviewState{}, err
	}

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
	if err := chromedp.Run(opCtx,
		setDocContent(html),
		waitMount(mountTimeout),
	); err != nil {
		return service.PreviewState{}, fmt.Errorf("load preview: %w", err)
	}
	return m.captureState(opCtx, s, pageID), nil
}

func (m *PreviewSessions) Navigate(ctx context.Context, pageID, route string) (service.PreviewState, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()
	if err := chromedp.Run(opCtx,
		chromedp.Evaluate("location.hash = "+jsString(normalizeRoute(route)), nil),
		chromedp.Sleep(settleDelay),
		waitMount(mountTimeout),
	); err != nil {
		return service.PreviewState{}, fmt.Errorf("navigate: %w", err)
	}
	return m.captureState(opCtx, s, pageID), nil
}

func (m *PreviewSessions) Snapshot(ctx context.Context, pageID string) (service.PreviewState, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()
	var outline string
	if err := chromedp.Run(opCtx, chromedp.Evaluate(snapshotJS, &outline)); err != nil {
		return service.PreviewState{}, fmt.Errorf("snapshot: %w", err)
	}
	st := m.captureState(opCtx, s, pageID)
	st.Outline = outline
	return st, nil
}

func (m *PreviewSessions) Screenshot(ctx context.Context, pageID string) ([]byte, service.PreviewState, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()
	var buf []byte
	if err := chromedp.Run(opCtx,
		chromedp.EmulateViewport(viewportW, viewportH),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return nil, service.PreviewState{}, fmt.Errorf("screenshot: %w", err)
	}
	return buf, m.captureState(opCtx, s, pageID), nil
}

func (m *PreviewSessions) Eval(ctx context.Context, pageID, expr string) (service.PreviewState, error) {
	if strings.TrimSpace(expr) == "" {
		return service.PreviewState{}, service.BadRequest("expr is required")
	}
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()
	var out string
	if err := chromedp.Run(opCtx, chromedp.Evaluate(wrapEval(expr), &out)); err != nil {
		return service.PreviewState{}, fmt.Errorf("eval: %w", err)
	}
	st := m.captureState(opCtx, s, pageID)
	st.EvalResult = out
	return st, nil
}

func (m *PreviewSessions) Click(ctx context.Context, pageID, selector string) (service.PreviewState, error) {
	if strings.TrimSpace(selector) == "" {
		return service.PreviewState{}, service.BadRequest("selector is required")
	}
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()
	// Existence check first so a bad selector returns a clear error instead of
	// hanging until the op timeout (chromedp.Click waits for the node).
	var exists bool
	if err := chromedp.Run(opCtx, chromedp.Evaluate("!!document.querySelector("+jsString(selector)+")", &exists)); err != nil {
		return service.PreviewState{}, fmt.Errorf("click: %w", err)
	}
	if !exists {
		return service.PreviewState{}, service.BadRequest("no element matches selector " + selector)
	}
	if err := chromedp.Run(opCtx,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Sleep(settleDelay),
	); err != nil {
		return service.PreviewState{}, fmt.Errorf("click: %w", err)
	}
	return m.captureState(opCtx, s, pageID), nil
}

func (m *PreviewSessions) Console(ctx context.Context, pageID string) (service.PreviewState, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return service.PreviewState{}, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()
	return m.captureState(opCtx, s, pageID), nil
}

func (m *PreviewSessions) Close(ctx context.Context, pageID string) error {
	key, err := sessionKey(ctx, pageID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[key]; ok {
		s.tabCancel()
		delete(m.sessions, key)
	}
	return nil
}

func (m *PreviewSessions) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		s.tabCancel()
		delete(m.sessions, k)
	}
	if m.allocCancel != nil {
		m.allocCancel()
		m.allocCancel = nil
		m.allocReady = false
	}
	if m.vendorSrv != nil {
		_ = m.vendorSrv.Close()
		m.vendorSrv = nil
	}
	m.stopReaperOnce.Do(func() { close(m.reaperStop) })
	return nil
}

// previewHTML builds the preview document. For an ESM build (importmap.json
// present), it absolutizes the /vendor URLs to the local vendor server and widens
// the meta-CSP to that origin; otherwise it serves the legacy inline doc.
func (m *PreviewSessions) previewHTML(ctx context.Context, pageID, css, js string) (string, error) {
	var im ImportMap
	csp := CSP
	if data, derr := m.store.ReadFile(ctx, pageID, importMapPath); derr == nil {
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
	return PreviewHTML(pageID, TokensCSS, css, js, csp, im), nil
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

func (m *PreviewSessions) getOrCreate(key string) (*previewSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureAllocLocked(); err != nil {
		return nil, err
	}
	if s, ok := m.sessions[key]; ok {
		s.lastUsed = time.Now()
		return s, nil
	}
	if len(m.sessions) >= m.maxSessions {
		m.evictLRULocked()
	}
	tabCtx, tabCancel := chromedp.NewContext(m.allocCtx)
	s := &previewSession{key: key, tabCtx: tabCtx, tabCancel: tabCancel, lastUsed: time.Now()}
	// Attach console/exception capture BEFORE the first load so nothing is missed.
	chromedp.ListenTarget(tabCtx, s.onEvent)
	m.sessions[key] = s
	return s, nil
}

func (m *PreviewSessions) getExisting(ctx context.Context, pageID string) (*previewSession, error) {
	key, err := sessionKey(ctx, pageID)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initErr != nil {
		return nil, unavailable(m.initErr)
	}
	s, ok := m.sessions[key]
	if !ok {
		return nil, service.BadRequest("no preview open for this page — call preview_open first")
	}
	s.lastUsed = time.Now()
	return s, nil
}

// ensureAllocLocked lazily creates the shared browser allocator. Caller holds m.mu.
func (m *PreviewSessions) ensureAllocLocked() error {
	if m.allocReady {
		return nil
	}
	if m.initErr != nil {
		return unavailable(m.initErr)
	}
	chromePath, err := resolveChrome()
	if err != nil {
		m.initErr = err
		return unavailable(err)
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		// The preview loads about:blank (an opaque "public" origin) and fetches the
		// local vendor server; disable Local/Private Network Access checks so those
		// loads aren't blocked. (Production loads the doc FROM the API origin, so
		// it's local->local and this never applies there.)
		chromedp.Flag("disable-features", "LocalNetworkAccessChecks,PrivateNetworkAccessChecks,BlockInsecurePrivateNetworkRequests"),
	)
	m.allocCtx, m.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	m.allocReady = true
	return nil
}

// evictLRULocked closes the least-recently-used tab. Caller holds m.mu.
func (m *PreviewSessions) evictLRULocked() {
	var oldestKey string
	var oldest time.Time
	for k, s := range m.sessions {
		if oldestKey == "" || s.lastUsed.Before(oldest) {
			oldestKey, oldest = k, s.lastUsed
		}
	}
	if oldestKey != "" {
		m.sessions[oldestKey].tabCancel()
		delete(m.sessions, oldestKey)
	}
}

func (m *PreviewSessions) reaper() {
	t := time.NewTicker(m.tickInterval)
	defer t.Stop()
	for {
		select {
		case <-m.reaperStop:
			return
		case <-t.C:
			m.mu.Lock()
			now := time.Now()
			for k, s := range m.sessions {
				if now.Sub(s.lastUsed) <= m.idleTTL {
					continue
				}
				// Never reap a tab with an op in flight (e.g. a slow browser
				// launch). TryLock never blocks, so we just skip until next tick.
				if !s.opMu.TryLock() {
					continue
				}
				s.tabCancel()
				delete(m.sessions, k)
				s.opMu.Unlock()
			}
			m.mu.Unlock()
		}
	}
}

// captureState reads url/title/mounted from the tab and drains the accumulated
// console/exception lines. Best-effort: a dead tab yields a zero-ish state.
func (m *PreviewSessions) captureState(opCtx context.Context, s *previewSession, pageID string) service.PreviewState {
	var snap struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Mounted bool   `json:"mounted"`
	}
	_ = chromedp.Run(opCtx, chromedp.Evaluate(stateJS, &snap))
	st := service.PreviewState{
		PageID:  pageID,
		URL:     snap.URL,
		Title:   snap.Title,
		Mounted: snap.Mounted,
	}
	st.Console, st.Exceptions = s.drainLogs()
	return st
}

// --- session event capture -------------------------------------------------

func (s *previewSession) onEvent(ev any) {
	switch e := ev.(type) {
	case *cdpruntime.EventConsoleAPICalled:
		line := string(e.Type) + ": " + formatConsoleArgs(e.Args)
		s.logMu.Lock()
		s.console = appendCapped(s.console, line, maxLogLines)
		s.logMu.Unlock()
	case *cdpruntime.EventExceptionThrown:
		msg := e.ExceptionDetails.Text
		if ex := e.ExceptionDetails.Exception; ex != nil && ex.Description != "" {
			msg = ex.Description
		}
		s.logMu.Lock()
		s.exceptions = appendCapped(s.exceptions, msg, maxLogLines)
		s.logMu.Unlock()
	}
}

func (s *previewSession) resetLogs() {
	s.logMu.Lock()
	s.console = nil
	s.exceptions = nil
	s.logMu.Unlock()
}

func (s *previewSession) drainLogs() (console, exceptions []string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return lastN(s.console, logReturn), lastN(s.exceptions, logReturn)
}

// --- helpers ---------------------------------------------------------------

// runFirst runs the tab's first chromedp action set on the RAW tab context (no
// timeout — a timeout on the first Run stops the whole browser per chromedp).
// A watchdog bounds it: if the browser hangs starting, the tab is cancelled and
// an error returned instead of blocking forever.
func runFirst(s *previewSession, timeout time.Duration, actions ...chromedp.Action) error {
	done := make(chan error, 1)
	go func() { done <- chromedp.Run(s.tabCtx, actions...) }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		s.tabCancel()
		return errors.New("browser start timed out")
	}
}

func sessionKey(ctx context.Context, pageID string) (string, error) {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	uid := strings.TrimSpace(p.UserID)
	pid := strings.TrimSpace(pageID)
	if uid == "" || pid == "" {
		return "", service.BadRequest("page_id is required")
	}
	return uid + "/" + pid, nil
}

func unavailable(err error) error {
	return service.BadRequest("renderer unavailable: " + err.Error() +
		" — preview tools require a Chrome/Chromium binary (set DOCSURFACE_CHROME_PATH)")
}

// resolveChrome finds a Chrome/Chromium binary: the env override, then common
// install locations, then $PATH. Returns an error (→ "renderer unavailable")
// when none is found.
func resolveChrome() (string, error) {
	if p := strings.TrimSpace(os.Getenv("DOCSURFACE_CHROME_PATH")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("DOCSURFACE_CHROME_PATH=%q not found", p)
	}
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no Chrome/Chromium binary found")
}

// normalizeRoute coerces an agent-supplied route into a hash route: "section",
// "/section", and "#/section" all become "#/section"; empty → "#/".
func normalizeRoute(r string) string {
	r = strings.TrimSpace(r)
	if r == "" {
		return "#/"
	}
	if strings.HasPrefix(r, "#") {
		return r
	}
	if strings.HasPrefix(r, "/") {
		return "#" + r
	}
	return "#/" + r
}

// wrapEval makes any agent expression return a JSON string (or "undefined" /
// "error: ...") so the result is always a readable scalar for the agent.
func wrapEval(expr string) string {
	return "(function(){try{var v=(" + expr +
		");return (typeof v==='undefined')?'undefined':JSON.stringify(v);}" +
		"catch(e){return 'error: '+((e&&e.message)||String(e));}})()"
}

// jsString renders s as a safely-quoted JS string literal.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func formatConsoleArgs(args []*cdpruntime.RemoteObject) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, formatConsoleArg(a))
	}
	return strings.Join(parts, " ")
}

func formatConsoleArg(a *cdpruntime.RemoteObject) string {
	if a == nil {
		return ""
	}
	if len(a.Value) > 0 {
		var v any
		if err := json.Unmarshal(a.Value, &v); err == nil {
			return fmt.Sprintf("%v", v)
		}
		return string(a.Value)
	}
	if a.Description != "" {
		return a.Description
	}
	return string(a.Type)
}

func appendCapped(s []string, v string, max int) []string {
	s = append(s, v)
	if len(s) > max {
		s = s[len(s)-max:]
	}
	return s
}

func lastN(s []string, n int) []string {
	if len(s) == 0 {
		return nil
	}
	if len(s) <= n {
		out := make([]string, len(s))
		copy(out, s)
		return out
	}
	out := make([]string, n)
	copy(out, s[len(s)-n:])
	return out
}

// --- JS snippets -----------------------------------------------------------

const stateJS = `({url:location.href,title:document.title,` +
	`mounted:!!document.getElementById('root')&&document.getElementById('root').childElementCount>0})`

// snapshotJS returns the current view's innerText (capped) plus a compact,
// depth-limited element outline (tag#id.class "own text").
const snapshotJS = `(function(){
  function walk(el,depth,lines){
    if(depth>6||lines.length>=200) return;
    for(var i=0;i<el.children.length;i++){
      if(lines.length>=200) return;
      var c=el.children[i];
      var s=c.tagName.toLowerCase();
      if(c.id) s+='#'+c.id;
      if(typeof c.className==='string'&&c.className.trim()) s+='.'+c.className.trim().split(/\s+/).slice(0,3).join('.');
      var own='';
      for(var j=0;j<c.childNodes.length;j++){var n=c.childNodes[j];if(n.nodeType===3){own+=' '+n.textContent.trim();}}
      own=own.trim();
      if(own) s+=' "'+own.slice(0,60)+'"';
      lines.push(new Array(depth+1).join('  ')+s);
      walk(c,depth+1,lines);
    }
  }
  var lines=[];walk(document.body,0,lines);
  var text=(document.body.innerText||'').slice(0,2000);
  return 'TEXT:\n'+text+'\n\nDOM:\n'+lines.join('\n');
})()`

// setDocContent replaces the tab's document with html (the frame id comes from
// the live frame tree, which requires a prior navigation to establish it).
func setDocContent(html string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		ft, err := page.GetFrameTree().Do(ctx)
		if err != nil {
			return err
		}
		return page.SetDocumentContent(ft.Frame.ID, html).Do(ctx)
	})
}

// waitMount polls for React to render into #root, up to timeout. Best-effort:
// it never errors — Mounted=false is itself useful feedback for the agent.
func waitMount(timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		deadline := time.Now().Add(timeout)
		for {
			var n int
			if err := chromedp.Evaluate(`(document.getElementById('root')||{childElementCount:0}).childElementCount`, &n).Do(ctx); err != nil {
				return nil
			}
			if n > 0 || time.Now().After(deadline) {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(50 * time.Millisecond):
			}
		}
	})
}
