package docsurface

import (
	"bytes"
	"context"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

// fixtureBundleJS is a hand-written (no-esbuild, no-network) "bundle" that mounts
// a tiny hash-routed app into #root: HOME on #/, SECOND on #/two, a button that
// navigates, a button that throws (to exercise exception capture), and a console
// log on each render. It lets the preview subsystem be tested without esm.sh.
const fixtureBundleJS = `
(function(){
  function render(){
    var r = document.getElementById('root');
    if(!r) return;
    var route = location.hash || '#/';
    r.innerHTML = '';
    var d = document.createElement('div');
    d.id = 'view-root';
    d.setAttribute('data-route', route);
    d.textContent = (route === '#/two') ? 'SECOND PAGE' : 'HOME PAGE';
    r.appendChild(d);
    var go = document.createElement('button');
    go.id = 'go';
    go.textContent = 'go';
    go.onclick = function(){ location.hash = '#/two'; };
    r.appendChild(go);
    var boom = document.createElement('button');
    boom.id = 'boom';
    boom.textContent = 'boom';
    boom.onclick = function(){ throw new Error('kaboom'); };
    r.appendChild(boom);
    console.log('rendered', route);
  }
  window.addEventListener('hashchange', render);
  render();
})();
`

func chromeAvailable(t *testing.T) {
	t.Helper()
	if _, err := resolveChrome(); err != nil {
		t.Skipf("no Chrome/Chromium available: %v", err)
	}
}

type fakeRuntime struct{ builds int }

func (f *fakeRuntime) Build(ctx context.Context, pageID string, channel service.BuildChannel) (service.BuildResult, error) {
	f.builds++
	return service.BuildResult{OK: true, ServedURL: "/content/" + pageID + "/"}, nil
}

func (f *fakeRuntime) ReadVendor(sha string) ([]byte, error) { return nil, service.ErrNotFound }

// newPreviewFixture wires the real filesystem store (seeded with the fixture
// bundle) to a no-op build runtime, so Open loads the fixture without a network.
func newPreviewFixture(t *testing.T, opts PreviewOptions, pages ...string) (*PreviewSessions, context.Context) {
	t.Helper()
	if len(pages) == 0 {
		pages = []string{"p1"}
	}
	root := t.TempDir()
	st := NewStore(root)
	ctx := testCtx()
	for _, pid := range pages {
		if _, err := st.EnsurePageDir(ctx, pid); err != nil {
			t.Fatalf("EnsurePageDir: %v", err)
		}
		if err := st.WriteFile(ctx, pid, "dist/bundle.js", []byte(fixtureBundleJS)); err != nil {
			t.Fatalf("WriteFile bundle: %v", err)
		}
	}
	m := NewPreviewSessions(st, &fakeRuntime{}, opts).(*PreviewSessions)
	t.Cleanup(func() { _ = m.CloseAll(context.Background()) })
	return m, ctx
}

func hasLine(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func TestPreviewSession_Interactive(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})

	// Open: React-equivalent mount, console captured, no exceptions.
	st, err := m.Open(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !st.Mounted {
		t.Fatalf("not mounted after Open: %+v", st)
	}
	if !hasLine(st.Console, "rendered") {
		t.Errorf("console missing render log: %v", st.Console)
	}
	if len(st.Exceptions) != 0 {
		t.Errorf("unexpected exceptions after Open: %v", st.Exceptions)
	}

	// Snapshot the home route.
	snap, err := m.Snapshot(ctx, "p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !strings.Contains(snap.Outline, "HOME PAGE") {
		t.Errorf("home outline missing HOME PAGE: %q", snap.Outline)
	}

	// Navigate to a second hash route → a genuinely different view.
	nav, err := m.Navigate(ctx, "p1", "#/two")
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if !strings.Contains(nav.URL, "#/two") {
		t.Errorf("URL after navigate = %q, want it to contain #/two", nav.URL)
	}
	snap2, err := m.Snapshot(ctx, "p1")
	if err != nil {
		t.Fatalf("Snapshot 2: %v", err)
	}
	if !strings.Contains(snap2.Outline, "SECOND PAGE") {
		t.Errorf("route-two outline missing SECOND PAGE: %q", snap2.Outline)
	}
	if strings.Contains(snap2.Outline, "HOME PAGE") {
		t.Errorf("route-two still shows HOME PAGE (routing not applied): %q", snap2.Outline)
	}

	// Eval: scalar + a DOM query.
	ev, err := m.Eval(ctx, "p1", "1+2")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if ev.EvalResult != "3" {
		t.Errorf("eval 1+2 = %q, want 3", ev.EvalResult)
	}
	ev2, err := m.Eval(ctx, "p1", "document.querySelectorAll('button').length")
	if err != nil {
		t.Fatalf("Eval 2: %v", err)
	}
	if ev2.EvalResult != "2" {
		t.Errorf("button count = %q, want 2", ev2.EvalResult)
	}

	// Screenshot: a decodable PNG.
	shot, _, err := m.Screenshot(ctx, "p1")
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(shot)); err != nil {
		t.Errorf("screenshot is not a valid PNG (%d bytes): %v", len(shot), err)
	}

	// Click the throwing button → the uncaught error is captured.
	if _, err := m.Click(ctx, "p1", "#boom"); err != nil {
		t.Fatalf("Click boom: %v", err)
	}
	con, err := m.Console(ctx, "p1")
	if err != nil {
		t.Fatalf("Console: %v", err)
	}
	if !hasLine(con.Exceptions, "kaboom") {
		t.Errorf("exception not captured after click: %v", con.Exceptions)
	}
}

// TestPreviewSession_VendoredDoc is the end-to-end vendored-deps regression: a
// REAL build externalizes react (→ /vendor + import map), and the preview's local
// vendor server + the LNA-disable flag must let the opaque-origin doc load those
// modules and mount. "mounted + no exceptions + rendered content" implies the
// import map resolved, the vendor server was reachable, react was shared (no
// invalid-hook error), and no CSP/LNA block occurred.
func TestPreviewSession_VendoredDoc(t *testing.T) {
	if !esmReachable() {
		t.Skip("esm.sh unreachable")
	}
	chromeAvailable(t)
	root := t.TempDir()
	st := NewStore(root)
	rt := NewBuilder(st, filepath.Join(root, "cache", "esm")) // real builder — vendors deps
	ctx := testCtx()
	if _, err := st.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	src := `import { useState } from "react";
import { createRoot } from "react-dom/client";
function App(){ const [n]=useState(7); return <div><h1 id="title">vendored {n}</h1></div>; }
createRoot(document.getElementById("root")).render(<App/>);`
	if err := st.WriteFile(ctx, "p1", "index.tsx", []byte(src)); err != nil {
		t.Fatal(err)
	}

	m := NewPreviewSessions(st, rt, PreviewOptions{}).(*PreviewSessions)
	t.Cleanup(func() { _ = m.CloseAll(context.Background()) })

	state, err := m.Open(ctx, "p1", service.ChannelPublished) // Open rebuilds (vendors) then loads via the import map
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !state.Mounted {
		t.Fatalf("vendored doc did not mount — import map / vendor server / react sharing broke: %+v", state)
	}
	if len(state.Exceptions) > 0 {
		t.Fatalf("exceptions in vendored doc: %v", state.Exceptions)
	}
	snap, err := m.Snapshot(ctx, "p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !strings.Contains(snap.Outline, "vendored 7") {
		t.Errorf("vendored doc did not render expected content: %q", snap.Outline)
	}
}

func TestPreviewSession_ClickMissingSelector(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})
	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err := m.Click(ctx, "p1", "#does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "no element matches") {
		t.Fatalf("want 'no element matches' error, got %v", err)
	}
}

// Ops before Open should return a clear, fast error — no browser launch needed.
func TestPreviewSession_RequiresOpen(t *testing.T) {
	m, ctx := newPreviewFixture(t, PreviewOptions{})
	_, err := m.Snapshot(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "preview_open first") {
		t.Fatalf("want 'preview_open first', got %v", err)
	}
}

func TestPreviewSession_IdleReaper(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{IdleTTL: 150 * time.Millisecond})
	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("Open: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		n := len(m.sessions)
		m.mu.Unlock()
		if n == 0 {
			return // reaped
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("idle session was not reaped within the deadline")
}

func TestPreviewSession_MaxSessionsEvict(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{MaxSessions: 1}, "p1", "p2")
	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("Open p1: %v", err)
	}
	if _, err := m.Open(ctx, "p2", service.ChannelPublished); err != nil {
		t.Fatalf("Open p2: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sessions) != 1 {
		t.Fatalf("want 1 session after cap+evict, got %d", len(m.sessions))
	}
	if _, ok := m.sessions["user-abc/p2"]; !ok {
		t.Fatalf("p2 should be the surviving (most-recent) session")
	}
}

func TestPreviewSession_CloseAndCloseAll(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})
	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Close(ctx, "p1"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	m.mu.Lock()
	n := len(m.sessions)
	m.mu.Unlock()
	if n != 0 {
		t.Fatalf("session not removed after Close: %d", n)
	}
	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	if err := m.CloseAll(ctx); err != nil { // t.Cleanup will call it again — must be safe.
		t.Fatalf("CloseAll: %v", err)
	}
}

// A crashed shared browser must self-heal: the next Open transparently rebuilds
// the renderer and mounts again, instead of returning "context canceled" forever
// (the failure that previously required a full mcp restart).
func TestPreviewSession_SelfHealsDeadBrowser(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})

	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Simulate a browser crash: cancel the shared allocator out from under the
	// live tab. This kills allocCtx and every child tabCtx.
	m.mu.Lock()
	allocCancel := m.allocCancel
	m.mu.Unlock()
	if allocCancel == nil {
		t.Fatal("expected an allocator after first Open")
	}
	allocCancel()
	// Let chromedp propagate the cancellation to the tab contexts.
	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		dead := m.allocCtx != nil && m.allocCtx.Err() != nil
		m.mu.Unlock()
		if dead || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A second Open must rebuild the browser and mount again.
	st, err := m.Open(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Fatalf("Open after crash should self-heal, got: %v", err)
	}
	if !st.Mounted {
		t.Fatalf("expected mounted after self-heal, state=%+v", st)
	}
}

// Reset (the preview_restart escape hatch) clears all sessions + the browser,
// and the next Open transparently rebuilds and mounts.
func TestPreviewSession_ResetThenReopen(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})

	if _, err := m.Open(ctx, "p1", service.ChannelPublished); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Reset(ctx); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	m.mu.Lock()
	n, ready := len(m.sessions), m.allocReady
	m.mu.Unlock()
	if n != 0 || ready {
		t.Fatalf("after Reset want 0 sessions & alloc not ready, got sessions=%d ready=%v", n, ready)
	}

	st, err := m.Open(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Fatalf("Open after Reset should rebuild, got: %v", err)
	}
	if !st.Mounted {
		t.Fatalf("expected mounted after reset+reopen, state=%+v", st)
	}
}

// No Chrome binary → every op degrades to a clean BadRequest, never a panic.
func TestPreviewSession_RendererUnavailable(t *testing.T) {
	t.Setenv("DOCSURFACE_CHROME_PATH", filepath.Join(t.TempDir(), "no-such-chrome"))
	m, ctx := newPreviewFixture(t, PreviewOptions{})
	_, err := m.Open(ctx, "p1", service.ChannelPublished)
	if err == nil || !strings.Contains(err.Error(), "renderer unavailable") {
		t.Fatalf("want 'renderer unavailable', got %v", err)
	}
	if _, err := m.Snapshot(ctx, "p1"); err == nil {
		t.Fatal("expected an error from Snapshot with no renderer")
	}
}
