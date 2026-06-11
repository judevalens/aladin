package docsurface

import (
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

// TokensCSS mirrors the Aladin dark-theme design tokens from
// aladin_react/src/index.css (:root). Served as /content/{id}/tokens.css and
// linked from the entry HTML so agent pages compose against var(--ink) etc. and
// inherit the app's look. Keep in sync with index.css manually (v1).
const TokensCSS = `:root{
  /* fonts */
  --font-display:"Space Grotesk Variable",system-ui,sans-serif;
  --font-sans:system-ui,-apple-system,"Segoe UI",sans-serif;
  --font-mono:"JetBrains Mono",ui-monospace,"SF Mono",monospace;
  /* surfaces */
  --rail:#0b0b0e; --panel:#0f0f12; --bg:#0d0d10; --chrome:#0b0b0e;
  --field:#161619; --card:#121215; --raise:#17171c; --explorer:#101013;
  /* ink ramp */
  --ink:#eceaef; --ink-2:#9694a0; --ink-3:#615f6b; --ink-4:#403e48;
  /* lines (rgb channels for rgb(var(--line)/alpha)) */
  --line:255 255 255 / 0.07; --line-2:255 255 255 / 0.045;
  /* accent + semantic hues */
  --amber:#c9925a; --for:#5cba8f; --against:#d8796b;
  --catalyst:#5b9bd8; --echo:#9a8cd8; --neutral:#7f8aa0;
  /* radii */
  --radius-chip:7px; --radius-card:12px; --radius-modal:14px;
}
html,body{margin:0;padding:0;background:var(--bg);color:var(--ink);font-family:var(--font-sans);}
`

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
	css := "<style>" + tokensCSS + "</style>"
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
