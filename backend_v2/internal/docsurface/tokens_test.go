package docsurface

import (
	"strings"
	"testing"
)

func TestPreviewHTMLCarriesMetaCSP(t *testing.T) {
	html := PreviewHTML("My Page", TokensCSS, "body{color:red}", `console.log("hi")`, CSP, ImportMap{}, "")

	// The policy must travel inline as a meta tag (SetDocumentContent has no headers).
	wantMeta := `<meta http-equiv="Content-Security-Policy" content="` + CSP + `">`
	if !strings.Contains(html, wantMeta) {
		t.Fatalf("PreviewHTML missing meta-CSP.\nwant substring: %s\n---\n%s", wantMeta, html)
	}
	// Inline allowances stay; network is open (connect-src https:) for external
	// resources; remote code stays closed (no unsafe-eval).
	for _, frag := range []string{"script-src 'unsafe-inline'", "style-src 'unsafe-inline'", "connect-src https:"} {
		if !strings.Contains(CSP, frag) {
			t.Errorf("CSP missing %q: %s", frag, CSP)
		}
	}
	if strings.Contains(CSP, "unsafe-eval") {
		t.Errorf("CSP must not allow unsafe-eval: %s", CSP)
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
	entry := EntryHTML("T", TokensCSS, "", `1`, ImportMap{}, "")
	preview := PreviewHTML("T", TokensCSS, "", `1`, CSP, ImportMap{}, "")
	// PreviewHTML = EntryHTML + three preview-only head injections: the CSP meta,
	// the unhandled-rejection capture, and the bridge emulator. Removing exactly
	// that block should recover EntryHTML.
	injected := "<meta http-equiv=\"Content-Security-Policy\" content=\"" + CSP + "\">\n" +
		"<script>" + previewRejectionCaptureJS + "</script>\n" +
		"<script>" + breakInlineClosers(previewBridgeEmulatorJS) + "</script>\n"
	if got := strings.Replace(preview, injected, "", 1); got != entry {
		t.Fatalf("PreviewHTML is not EntryHTML+meta+emulator:\n--- recovered ---\n%s\n--- entry ---\n%s", got, entry)
	}
}

// TestEntryHTMLImportMap: an ESM build (non-nil Imports) emits an import map +
// module script; a legacy build (nil Imports) stays a bare classic script.
func TestEntryHTMLImportMap(t *testing.T) {
	im := ImportMap{Imports: map[string]string{"react": "/vendor/abc123", "react-dom/client": "/vendor/def456"}}
	esm := EntryHTML("T", TokensCSS, "", `createRoot()`, im, "")
	for _, frag := range []string{`<script type="importmap">`, `<script type="module">`, `/vendor/abc123`, `"react-dom/client"`} {
		if !strings.Contains(esm, frag) {
			t.Errorf("ESM EntryHTML missing %q", frag)
		}
	}
	legacy := EntryHTML("T", TokensCSS, "", `var x=1`, ImportMap{}, "")
	if strings.Contains(legacy, "importmap") || strings.Contains(legacy, `type="module"`) {
		t.Errorf("legacy (nil import map) must be a bare <script>:\n%s", legacy)
	}
	if !strings.Contains(legacy, "<script>var x=1</script>") {
		t.Errorf("legacy bare script missing")
	}

	// CSPWithVendor widens script-src by exactly the origin (for /vendor modules).
	csp := CSPWithVendor("http://localhost:8000")
	if !strings.Contains(csp, "script-src 'unsafe-inline' http://localhost:8000") || !strings.Contains(csp, "connect-src https:") {
		t.Errorf("CSPWithVendor wrong: %s", csp)
	}
}
