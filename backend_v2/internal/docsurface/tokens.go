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

// CSP for shard pages. The security boundary is FRAME ISOLATION: the iframe is
// sandboxed to an opaque origin (no allow-same-origin), so the shard cannot
// reach Aladin's DOM, cookies, session, or other artifacts. That containment
// holds regardless of trust and bounds the blast radius of bugs.
//
// Network + display resources are OPEN (connect-src / img / font / media / frame
// to https): the trust decision is the MCP grant on a trusted machine — a client
// you've authorized already has full workspace access, so locking the shard's
// network defended against nothing real while crippling the surface. Remote CODE
// stays closed: script-src is 'unsafe-inline' + (via CSPWithVendor) the vendored
// origin only — no unsafe-eval, no arbitrary runtime scripts; deps go through the
// build/vendor pipeline. base-uri/form-action stay locked (cheap; forms are moot
// under sandbox="allow-scripts").
//
// COMPANION (land with the bridge): the view-time access_token rides the shard
// URL and is readable by shard JS; with connect-src open, scope it to /content
// so a shard can't raw-call /api as the viewer — WORKSPACE data flows through the
// bridge (declared refs + provenance); the open network is for EXTERNAL resources.
//
// The serve route sets this as an HTTP header; PreviewHTML mirrors it as a <meta>.
const CSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https:; img-src https: data: blob:; font-src https: data:; media-src https: data: blob:; connect-src https: wss:; frame-src https:; base-uri 'none'; form-action 'none'"

// themeCSS is the canonical Aladin design theme — the SINGLE source of truth,
// generated from aladin_react/src/theme.css via `make tokens` and drift-guarded
// by `make check-tokens`. It carries the Tailwind v4 `@theme inline` block: the
// runtime engine (KIT-1.2) compiles the utilities, inlining each token's base var
// directly — so it does NOT emit :root --color-* custom properties (that's what
// `inline` means). shardColorRootCSS re-emits them so var(--color-*) still
// resolves. No hand-authored token values live in Go.
//
//go:embed theme.css
var themeCSS string

// TokensCSS is the theme plus the shard document base, inlined into every shard.
var TokensCSS = themeCSS + `
html,body{margin:0;padding:0;background:var(--bg);color:var(--ink);font-family:var(--font-sans);}
`

// shardColorRootCSS mirrors the @theme tokens into a REAL :root block. theme.css
// uses Tailwind's `@theme inline`, which compiles tokens straight into utilities
// and deliberately does NOT emit :root custom properties — so var(--color-*) is
// undefined at runtime, and any chart/SVG/inline style that references a token
// color (recharts stroke/fill, hand-drawn SVG, a tok() resolver) renders with no
// color. EntryHTML re-emits the tokens as a plain :root (a normal <style>,
// applied by the browser independent of the runtime engine), so var(--color-*)
// resolves identically in the headless preview and when served. Shard-only:
// theme.css (host-shared, drift-guarded) is untouched.
var shardColorRootCSS = themeRootMirror(themeCSS)

// themeRootMirror lifts the body of the first @theme block into a :root selector.
// The @theme block contains only `--name: value;` declarations (no nested
// braces), so wrapping its body in :root{} is valid CSS.
func themeRootMirror(theme string) string {
	i := strings.Index(theme, "@theme")
	if i < 0 {
		return ""
	}
	open := strings.IndexByte(theme[i:], '{')
	if open < 0 {
		return ""
	}
	open += i
	end := strings.IndexByte(theme[open:], '}')
	if end < 0 {
		return ""
	}
	body := strings.TrimSpace(theme[open+1 : open+end])
	if body == "" {
		return ""
	}
	return ":root{" + body + "}"
}

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
		"<script>" + breakInlineClosers(tailwindBrowserJS) + "</script>" +
		// A plain :root mirror of the @theme tokens so var(--color-*) resolves at
		// runtime (see shardColorRootCSS): `@theme inline` does not emit these.
		"<style>" + shardColorRootCSS + "</style>"
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
