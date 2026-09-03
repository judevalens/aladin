package docsurface

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"aladin/backend_v2/internal/service"

	page "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func (m *PreviewSessions) getOrCreate(key string) (*previewSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureAllocLocked(); err != nil {
		return nil, err
	}
	if s, ok := m.sessions[key]; ok {
		// Reuse the tab only if it's still alive; a dead tab (browser restarted,
		// or this tab crashed while the browser survived) is dropped and rebuilt.
		if s.tabCtx.Err() == nil {
			s.lastUsed = time.Now()
			return s, nil
		}
		s.tabCancel()
		delete(m.sessions, key)
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
		// Self-heal: if the shared browser died (crash, OOM, external kill), its
		// context is canceled and every child tab is dead. Tear the whole thing
		// down here so we rebuild a fresh browser below instead of handing out
		// dead tabs forever (the failure mode that needed a full mcp restart).
		if m.allocCtx != nil && m.allocCtx.Err() == nil {
			return nil
		}
		m.resetBrowserLocked()
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

// resetBrowser tears down the shared browser + all tabs so the next Open
// rebuilds from scratch. Used to recover from a crashed renderer.
func (m *PreviewSessions) resetBrowser() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetBrowserLocked()
}

// resetBrowserLocked cancels every tab and the shared allocator, clearing the
// session map, so ensureAllocLocked rebuilds a fresh browser on the next call.
// It deliberately leaves the vendor server, the reaper, and a permanent
// initErr (no-Chrome) untouched. Caller holds m.mu.
func (m *PreviewSessions) resetBrowserLocked() {
	for k, s := range m.sessions {
		s.tabCancel()
		delete(m.sessions, k)
	}
	if m.allocCancel != nil {
		m.allocCancel()
	}
	m.allocCtx, m.allocCancel, m.allocReady = nil, nil, false
}

// isBrowserDead reports whether err means the browser/tab context died (a crash
// or external kill) — as opposed to a normal op timeout or an app-level failure.
// Such errors are recoverable by rebuilding the browser and retrying.
func isBrowserDead(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "browser start timed out")
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

// rendererUnavailableMsg prefixes the "no Chrome binary" BadRequest so callers
// can distinguish a missing renderer from a real failure (e.g. publish soft-warns
// instead of hard-gating when there's nothing to verify with).
const rendererUnavailableMsg = "renderer unavailable: "

func unavailable(err error) error {
	return service.BadRequest(rendererUnavailableMsg + err.Error() +
		" — preview tools require a Chrome/Chromium binary (set DOCSURFACE_CHROME_PATH)")
}

// IsRendererUnavailable reports whether err is the "no Chrome binary" condition,
// as opposed to a build failure or a genuine mount failure.
func IsRendererUnavailable(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), rendererUnavailableMsg)
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
