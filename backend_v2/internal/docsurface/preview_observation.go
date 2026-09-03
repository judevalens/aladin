package docsurface

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/service"

	"github.com/chromedp/chromedp"
)

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

// ConsoleErrors returns the console.error lines accumulated since open.
func (m *PreviewSessions) ConsoleErrors(ctx context.Context, pageID string) ([]string, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return nil, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.drainConsoleErrors(), nil
}

// CheckAnchors counts, for each declared anchor id, how many elements carry it
// on the CURRENT route (authored markup stamps data-anchor). This is the check the
// manifest always promised and never had: a shard whose anchors.json claims a
// region that isn't in the DOM is lying about its own structure, and every
// downstream consumer (deep links, provenance, live regions) is broken.
func (m *PreviewSessions) CheckAnchors(ctx context.Context, pageID string, anchorIDs []string) (map[string]int, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	if len(anchorIDs) == 0 {
		return out, nil
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()

	ids, err := json.Marshal(anchorIDs)
	if err != nil {
		return nil, err
	}
	// One round trip for every anchor. CSS.escape keeps a hostile/odd anchor id
	// from breaking the selector.
	expr := `(function(ids){var o={};ids.forEach(function(id){
	  o[id]=document.querySelectorAll('[data-anchor="'+CSS.escape(id)+'"]').length;});return o;})(` + string(ids) + `)`
	var counts map[string]int
	if err := chromedp.Run(opCtx, chromedp.Evaluate(expr, &counts)); err != nil {
		return nil, fmt.Errorf("check anchors: %w", err)
	}
	for id, n := range counts {
		out[id] = n
	}
	return out, nil
}

// EscapingLinks returns the hrefs on the CURRENT route that would navigate the
// frame off its own document. A served shard lives at
// /content/{id}/?access_token=… and that query token is its whole credential, so
// any href that is not a pure fragment replaces the shard with the API's
// {"error":"Unauthenticated"} the moment it is clicked. Fragment links, explicit
// schemes (mailto:/tel:/https:…) and javascript: are left alone — only in-app
// navigation that silently drops the credential is reported.
func (m *PreviewSessions) EscapingLinks(ctx context.Context, pageID string) ([]string, error) {
	s, err := m.getExisting(ctx, pageID)
	if err != nil {
		return nil, err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	opCtx, cancel := context.WithTimeout(s.tabCtx, opTimeout)
	defer cancel()

	// getAttribute (not .href) so the raw authored value is judged, before the
	// browser resolves it against the document URL.
	expr := `(function(){var out=[];
	  document.querySelectorAll('a[href]').forEach(function(a){
	    var h=(a.getAttribute('href')||'').trim();
	    if(!h) return;
	    if(h.charAt(0)==='#') return;              // in-app hash route: safe
	    if(/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(h)) return; // explicit scheme: deliberate
	    if(out.indexOf(h)<0) out.push(h);
	  });
	  return out.slice(0,20);})()`
	var hrefs []string
	if err := chromedp.Run(opCtx, chromedp.Evaluate(expr, &hrefs)); err != nil {
		return nil, fmt.Errorf("escaping links: %w", err)
	}
	return hrefs, nil
}
