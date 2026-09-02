package docsurface

import (
	"strings"
	"testing"
)

// Every theme the frontend defines must be parsed out of the embedded theme.css
// — a new [data-theme] block becomes servable with no Go change.
func TestValidThemeNames(t *testing.T) {
	for _, name := range []string{"dark", "soft", "cool", "contrast", "linear", "apple-dark", "apple-light", "light"} {
		if !ValidTheme(name) {
			t.Errorf("ValidTheme(%q) = false; theme.css should define it", name)
		}
	}
	for _, name := range []string{"", "neon", "DARK", "light "} {
		if ValidTheme(name) {
			t.Errorf("ValidTheme(%q) = true; want false", name)
		}
	}
}

func TestEntryHTMLThemeStamp(t *testing.T) {
	stamped := EntryHTML("t", TokensCSS, "", "1", ImportMap{}, "light")
	if !strings.Contains(stamped, `<html lang="en" data-theme="light">`) {
		t.Fatalf("EntryHTML with theme should stamp data-theme on <html>:\n%s", stamped[:200])
	}
	// No theme → a bare <html> tag (the CSS below still legitimately contains
	// [data-theme] selectors, so check the tag, not the whole document).
	plain := EntryHTML("t", TokensCSS, "", "1", ImportMap{}, "")
	if !strings.Contains(plain, "<html lang=\"en\">\n") {
		t.Fatalf("EntryHTML without theme must emit a bare <html> tag")
	}
}

func TestEntryHTMLAlwaysInstallsLiveThemeSync(t *testing.T) {
	doc := EntryHTML("t", TokensCSS, "", "1", ImportMap{}, "light")
	for _, want := range []string{`__aladinApplyThemePush`, `window.addEventListener("message"`, `m.channel!=="theme"`, `document.documentElement.dataset.theme=theme`} {
		if !strings.Contains(doc, want) {
			t.Fatalf("EntryHTML missing SDK-independent live theme sync %q", want)
		}
	}
}

// The plain-CSS tail makes the stamped theme apply at PARSE time (first paint),
// independent of the runtime Tailwind engine.
func TestShardThemeTailCSS(t *testing.T) {
	if strings.Contains(shardThemeTailCSS, "@theme") {
		t.Fatalf("theme tail must exclude the @theme block")
	}
	for _, sel := range []string{`[data-theme="dark"]`, `[data-theme="soft"]`, `[data-theme="light"]`} {
		if !strings.Contains(shardThemeTailCSS, sel) {
			t.Errorf("theme tail missing %s — did the theme blocks move out of theme.css?", sel)
		}
	}
	doc := EntryHTML("t", TokensCSS, "", "1", ImportMap{}, "light")
	if !strings.Contains(doc, shardThemeTailCSS) {
		t.Fatalf("EntryHTML must inline the plain theme tail")
	}
}

// The preview emulator must answer the shard SDK's theme handshake so themed shards
// behave headless. String-level guard: the emulator source names both methods.
func TestPreviewEmulatorSpeaksTheme(t *testing.T) {
	for _, method := range []string{`"hello"`, `"theme.get"`} {
		if !strings.Contains(previewBridgeEmulatorJS, method) {
			t.Errorf("preview emulator missing %s", method)
		}
	}
	doc := PreviewHTML("t", TokensCSS, "", "1", CSP, ImportMap{}, "soft")
	if !strings.Contains(doc, `data-theme="soft"`) {
		t.Fatalf("PreviewHTML must forward the theme stamp")
	}
}
