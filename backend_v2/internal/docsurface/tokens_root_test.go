package docsurface

import "strings"

import "testing"

// The @theme tokens must be re-emitted as a real :root block so var(--color-*)
// resolves at runtime (`@theme inline` does not emit them).
func TestShardColorRootMirror(t *testing.T) {
	if !strings.HasPrefix(strings.TrimSpace(shardColorRootCSS), ":root{") {
		t.Fatalf("mirror should be a :root block, got: %.40q", shardColorRootCSS)
	}
	for _, v := range []string{"--color-against", "--color-amber", "--color-ink", "--color-panel"} {
		if !strings.Contains(shardColorRootCSS, v) {
			t.Fatalf("mirror missing %s", v)
		}
	}
	// It must NOT carry the @theme wrapper (must be plain CSS the browser applies).
	if strings.Contains(shardColorRootCSS, "@theme") {
		t.Fatalf("mirror must not contain @theme")
	}
}

// EntryHTML must inline the :root mirror as a PLAIN <style> (not inside the
// text/tailwindcss block, which the browser ignores).
func TestEntryHTMLIncludesColorRoot(t *testing.T) {
	html := EntryHTML("t", TokensCSS, "", "console.log(1)", ImportMap{})
	if !strings.Contains(html, "<style>"+shardColorRootCSS+"</style>") {
		t.Fatalf("EntryHTML missing the plain :root color mirror style")
	}
}
