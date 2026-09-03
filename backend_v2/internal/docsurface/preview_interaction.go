package docsurface

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/service"

	"github.com/chromedp/chromedp"
)

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
