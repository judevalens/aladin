package docsurface

import (
	"strings"
	"testing"
)

func TestPreviewHTMLCarriesMetaCSP(t *testing.T) {
	html := PreviewHTML("My Page", TokensCSS, "body{color:red}", `console.log("hi")`)

	// The policy must travel inline as a meta tag (SetDocumentContent has no headers).
	wantMeta := `<meta http-equiv="Content-Security-Policy" content="` + CSP + `">`
	if !strings.Contains(html, wantMeta) {
		t.Fatalf("PreviewHTML missing meta-CSP.\nwant substring: %s\n---\n%s", wantMeta, html)
	}
	// The policy itself must keep the inline allowances + the no-exfil guard.
	for _, frag := range []string{"script-src 'unsafe-inline'", "style-src 'unsafe-inline'", "connect-src 'none'"} {
		if !strings.Contains(CSP, frag) {
			t.Errorf("CSP missing %q: %s", frag, CSP)
		}
	}
	// It must be the same self-contained document as the serve path: #root, the
	// inline bundle, the design tokens, and the page CSS.
	for _, frag := range []string{`id="root"`, `console.log(`, "--amber", "body{color:red}"} {
		if !strings.Contains(html, frag) {
			t.Errorf("PreviewHTML missing %q", frag)
		}
	}
}

// TestPreviewHTMLMatchesEntryBody guards faithfulness: PreviewHTML must be
// EntryHTML plus the meta tag and nothing else, so what the agent previews is
// what the iframe serves.
func TestPreviewHTMLMatchesEntryBody(t *testing.T) {
	entry := EntryHTML("T", TokensCSS, "", `1`)
	preview := PreviewHTML("T", TokensCSS, "", `1`)
	meta := "<meta http-equiv=\"Content-Security-Policy\" content=\"" + CSP + "\">\n"
	// Removing the injected meta line should recover EntryHTML exactly.
	if got := strings.Replace(preview, meta, "", 1); got != entry {
		t.Fatalf("PreviewHTML is not EntryHTML+meta:\n--- recovered ---\n%s\n--- entry ---\n%s", got, entry)
	}
}
