package docsurface

import (
	_ "embed"
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// ImportMap is the (optional) ES module import map for a built doc: bare specifier
// -> /vendor/<sha> URL (relative, so it resolves against the doc's base origin),
// plus best-effort SRI. A NIL Imports map means a legacy (inlined IIFE) build with
// no vendored deps — served the old way. A non-nil (possibly empty) map means an
// ESM build served as a module + import map.
type ImportMap struct {
	Imports   map[string]string `json:"imports"`
	Integrity map[string]string `json:"integrity,omitempty"`
}

// CSP is the Content-Security-Policy applied to every Doc Surface page. The page
// is fully self-contained: the design tokens, built CSS, and the IIFE bundle
// (React + deps) are all INLINED into the HTML, so nothing is loaded by URL. The
// frame is sandboxed to an opaque origin (no allow-same-origin), for which CSP
// 'self' matches nothing — hence 'unsafe-inline' for the inlined script/style,
// which is safe precisely because the frame is isolated: connect-src 'none' (no
// exfil), no DOM/cookie access to Aladin. The iframe sandbox is the security
// boundary. The serve route sets this as an HTTP header; PreviewHTML mirrors it
// as a <meta> tag so the headless preview renders under the identical policy.
const CSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; base-uri 'none'; form-action 'none'"

// themeCSS is the canonical Aladin design theme — the SINGLE source of truth,
// generated from aladin_react/src/theme.css via `make tokens` and drift-guarded
// by `make check-tokens`. It carries the Tailwind v4 @theme (which yields both
// the utility classes and the :root custom properties). In the shard's plain
// <style> the @theme at-rule is inert; once the shard's runtime Tailwind loads
// (KIT-1.2) it compiles the utilities. No hand-authored token values live in Go.
//
//go:embed theme.css
var themeCSS string

// TokensCSS is the theme plus the shard document base, inlined into every shard.
var TokensCSS = themeCSS + `
html,body{margin:0;padding:0;background:var(--bg);color:var(--ink);font-family:var(--font-sans);}
`

// tailwindBrowserJS is the @tailwindcss/browser v4 engine (a self-contained,
// WASM-free, network-free IIFE — verified: no fetch/eval/WebAssembly, so it runs
// under the shard CSP's connect-src 'none' + script-src 'unsafe-inline'). It
// scans the DOM + the <style type="text/tailwindcss"> block and JIT-injects the
// utilities agents use. KIT-1.2 "B" (runtime). The build-time path "A" (deferred)
// will compile Tailwind at build and drop this ~275KB/page runtime cost.
//
//go:embed tailwind-browser.js
var tailwindBrowserJS string

var reCloseScript = regexp.MustCompile(`(?i)</script`)
var reCloseStyle = regexp.MustCompile(`(?i)</style`)

// breakInlineClosers neutralizes any literal </script or </style inside inlined
// content so it can't break out of its <script>/<style> block. The \/ is inert
// in JS/CSS string contexts where such sequences would legitimately appear.
func breakInlineClosers(s string) string {
	s = reCloseScript.ReplaceAllString(s, `<\/script`)
	s = reCloseStyle.ReplaceAllString(s, `<\/style`)
	return s
}

// EntryHTML is the iframe document for a built Doc Surface page. Everything is
// INLINED — the design tokens, the built CSS, and the IIFE bundle — because the
// iframe is sandboxed to an OPAQUE origin (no allow-same-origin), for which CSP
// 'self' matches nothing, so same-host sub-resources can't be loaded. Inlining
// also means only the entry document needs auth (no sub-resource requests). The
// CSP ('unsafe-inline' script/style, connect-src 'none') is set by the serve route.
func EntryHTML(title, tokensCSS, bundleCSS, bundleJS string, im ImportMap) string {
	// Tailwind is the agent default styling: the theme/tokens go in a
	// type="text/tailwindcss" block (so the runtime engine compiles the @theme
	// into utilities), and the engine script follows it (it scans the DOM +
	// this block and JIT-injects used utilities). Both ride script-src/style-src
	// 'unsafe-inline'; the engine needs no network (connect-src 'none' holds).
	css := `<style type="text/tailwindcss">@import "tailwindcss";` + "\n" + tokensCSS + "</style>" +
		"<script>" + breakInlineClosers(tailwindBrowserJS) + "</script>"
	if bundleCSS != "" {
		css += "<style>" + breakInlineClosers(bundleCSS) + "</style>"
	}
	var scripts string
	if im.Imports == nil {
		// Legacy build: inlined IIFE bundle, no vendored deps.
		scripts = "<script>" + breakInlineClosers(bundleJS) + "</script>"
	} else {
		// ESM build: an import map (bare specifier -> /vendor/<sha>) then the module.
		imJSON, _ := json.Marshal(im)
		scripts = `<script type="importmap">` + breakInlineClosers(string(imJSON)) + "</script>\n" +
			`<script type="module">` + breakInlineClosers(bundleJS) + "</script>"
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
` + css + `
</head>
<body>
<div id="root"></div>
` + scripts + `
</body>
</html>`
}

// CSPWithVendor returns the doc CSP with the vendor origin added to script-src so
// the opaque-origin doc can load /vendor ES modules. connect-src 'none' still holds
// (module loads ride script-src, not connect-src), so no exfil path is opened.
func CSPWithVendor(origin string) string {
	if origin == "" {
		return CSP
	}
	return strings.Replace(CSP, "script-src 'unsafe-inline'", "script-src 'unsafe-inline' "+origin, 1)
}

// CSPWithVendorScheme widens script-src to allow the custom `vendor:` scheme, used
// by the desktop (Tauri) app where deps are served from a local cache via
// vendor://deps/<sha> instead of an HTTP origin. connect-src 'none' still holds.
func CSPWithVendorScheme() string {
	return strings.Replace(CSP, "script-src 'unsafe-inline'", "script-src 'unsafe-inline' vendor:", 1)
}

// PreviewHTML is EntryHTML for the headless preview renderer. It is byte-for-byte
// the same document the serve route emits, except it carries the CSP as a <meta
// http-equiv> tag: page.SetDocumentContent loads HTML with no HTTP headers, so
// the policy must travel inline for the preview to render under the SAME
// constraints as production (an agent that passes preview then passes the iframe).
func PreviewHTML(title, tokensCSS, bundleCSS, bundleJS, csp string, im ImportMap) string {
	doc := EntryHTML(title, tokensCSS, bundleCSS, bundleJS, im)
	meta := "<meta http-equiv=\"Content-Security-Policy\" content=\"" + csp + "\">\n"
	// Insert immediately after the charset meta so the policy is the first thing
	// the parser applies. EntryHTML always emits this exact line.
	return strings.Replace(doc, "<meta charset=\"utf-8\">\n", "<meta charset=\"utf-8\">\n"+meta, 1)
}

// NotBuiltHTML is shown when a page has no built bundle yet.
func NotBuiltHTML(title string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>` +
		html.EscapeString(title) + `</title><style>` + TokensCSS +
		`body{padding:32px;font-family:var(--font-sans)}</style></head>` +
		`<body><p style="color:var(--ink-2)">This page hasn't been built yet. Run build_app.</p></body></html>`
}
