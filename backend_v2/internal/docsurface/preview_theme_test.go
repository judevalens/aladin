package docsurface

import (
	"strings"
	"testing"

	"aladin/backend_v2/internal/service"
)

// End-to-end theme proof in a real renderer: opening the same page with
// different themes must change the PAINTED background — the stamped data-theme
// activates the [data-theme] var blocks, and body{background:var(--bg)} follows.
// This is the M1 risk spike as a durable regression test.
func TestPreviewSession_ThemeChangesPaint(t *testing.T) {
	chromeAvailable(t)
	m, ctx := newPreviewFixture(t, PreviewOptions{})

	bodyBG := func(theme string) string {
		if _, err := m.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{Theme: theme}); err != nil {
			t.Fatalf("Open(theme=%q): %v", theme, err)
		}
		st, err := m.Eval(ctx, "p1", "getComputedStyle(document.body).backgroundColor")
		if err != nil {
			t.Fatalf("Eval(theme=%q): %v", theme, err)
		}
		return st.EvalResult
	}

	dark := bodyBG("")       // default palette: --bg #0d0d10
	light := bodyBG("light") // light palette:   --bg #f7f6f3

	if !strings.Contains(dark, "13, 13, 16") {
		t.Errorf("default theme body background = %s; want rgb(13, 13, 16)", dark)
	}
	if !strings.Contains(light, "247, 246, 243") {
		t.Errorf("light theme body background = %s; want rgb(247, 246, 243)", light)
	}
	if dark == light {
		t.Fatalf("theme stamp had no effect on paint: both %s", dark)
	}

	// data-theme is visible to shard code and follows host pushes even though
	// this fixture does not import the shard SDK.
	st, err := m.Eval(ctx, "p1", "document.documentElement.getAttribute('data-theme')")
	if err != nil {
		t.Fatalf("Eval data-theme: %v", err)
	}
	if !strings.Contains(st.EvalResult, "light") {
		t.Errorf("data-theme attribute = %s; want light", st.EvalResult)
	}
	live, err := m.Eval(ctx, "p1", `(window.__aladinApplyThemePush({aladin:"bridge/1",type:"push",channel:"theme",data:{theme:"dark"}}),getComputedStyle(document.body).backgroundColor)`)
	if err != nil {
		t.Fatalf("Eval live theme push: %v", err)
	}
	if !strings.Contains(live.EvalResult, "13, 13, 16") {
		t.Errorf("live theme push did not repaint without SDK import: %s", live.EvalResult)
	}

	// Invalid theme names are dropped server-side → default palette.
	if got := bodyBG("neon"); !strings.Contains(got, "13, 13, 16") {
		t.Errorf("invalid theme should fall back to dark palette, got %s", got)
	}
}
