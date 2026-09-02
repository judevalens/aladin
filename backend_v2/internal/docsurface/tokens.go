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
// runtime Tailwind engine compiles the utilities, inlining each token's base var
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

// shardThemeTailCSS is everything in theme.css AFTER the @theme block — the
// keyframes, the dark base-var block, and every [data-theme] override block —
// re-emitted as a plain <style>. That CSS also rides the text/tailwindcss block
// (the engine passes non-@theme rules through), but the plain copy applies at
// PARSE time: with the serve-route's data-theme stamp, the first paint is in the
// right theme before the runtime engine has run at all. (The @custom-variant
// line in the tail is an unknown at-rule to the browser — skipped, harmless.)
var shardThemeTailCSS = themePlainTail(themeCSS)

// themePlainTail returns the content following the first @theme block's closing
// brace (the @theme body has no nested braces).
func themePlainTail(theme string) string {
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
	return strings.TrimSpace(theme[open+end+1:])
}

// reThemeName matches the [data-theme="…"] selectors in theme.css; the set of
// names is derived from the stylesheet itself so a new theme block is
// automatically servable with no Go change.
var reThemeName = regexp.MustCompile(`\[data-theme="([a-z-]+)"\]`)

// themeNames is the set of valid theme names parsed from the embedded theme.css.
var themeNames = func() map[string]bool {
	names := map[string]bool{}
	for _, m := range reThemeName.FindAllStringSubmatch(themeCSS, -1) {
		names[m[1]] = true
	}
	return names
}()

// ValidTheme reports whether name is a theme the embedded stylesheet defines.
// Serve callers use it to sanitize the ?theme= query param — an unknown name is
// dropped (the doc then falls back to the default dark palette).
func ValidTheme(name string) bool {
	return themeNames[name]
}

// tailwindBrowserJS is the @tailwindcss/browser v4 engine (a self-contained,
// WASM-free, network-free IIFE — verified: no fetch/eval/WebAssembly, so it runs
// under the shard CSP's connect-src 'none' + script-src 'unsafe-inline'). It
// scans the DOM + the <style type="text/tailwindcss"> block and JIT-injects the
// utilities agents use. The current path compiles at runtime; a deferred build-time
// path will compile Tailwind during the build and drop this ~275KB/page runtime cost.
//
//go:embed tailwind-browser.js
var tailwindBrowserJS string

var reCloseScript = regexp.MustCompile(`(?i)</script`)
var reCloseStyle = regexp.MustCompile(`(?i)</style`)

// themeSyncJS belongs to the document shell, not the optional data SDK. Every
// shard must follow live host theme changes, including a purely visual shard
// that imports only React. The first paint still comes from the server-stamped
// data-theme attribute; this listener applies later bridge pushes without a
// reload. Authored code can already change its own data-theme value, so source
// authentication adds no security boundary here; the bridge envelope and theme
// channel keep unrelated messages from changing it accidentally.
const themeSyncJS = `(function(){
  if(window.__aladinApplyThemePush)return;
  window.__aladinApplyThemePush=function(m){
    if(!m||(m.aladin!=="bridge/1"&&m.aladin!=="bridge/2")||m.type!=="push"||m.channel!=="theme")return;
    var theme=m.data&&m.data.theme;
    if(typeof theme==="string"&&theme)document.documentElement.dataset.theme=theme;
  };
  window.addEventListener("message",function(e){window.__aladinApplyThemePush(e.data);});
})();`

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
//
// theme, when non-empty (already validated via ValidTheme), is stamped as
// data-theme on <html> so the FIRST PAINT is in the viewer's theme; the host
// document-shell listener re-stamps live on later switches. Empty theme = default dark.
func EntryHTML(title, tokensCSS, bundleCSS, bundleJS string, im ImportMap, theme string) string {
	// Tailwind is the agent default styling: the theme/tokens go in a
	// type="text/tailwindcss" block (so the runtime engine compiles the @theme
	// into utilities), and the engine script follows it (it scans the DOM +
	// this block and JIT-injects used utilities). Both ride script-src/style-src
	// 'unsafe-inline'; the engine needs no network (connect-src 'none' holds).
	css := `<style type="text/tailwindcss">@import "tailwindcss";` + "\n" + tokensCSS + "</style>" +
		"<script>" + breakInlineClosers(tailwindBrowserJS) + "</script>" +
		// A plain :root mirror of the @theme tokens so var(--color-*) resolves at
		// runtime (see shardColorRootCSS): `@theme inline` does not emit these —
		// then the theme tail (base vars + every [data-theme] block) so the
		// stamped theme colors apply at parse time (see shardThemeTailCSS).
		"<style>" + shardColorRootCSS + "\n" + shardThemeTailCSS + "</style>"
	if bundleCSS != "" {
		css += "<style>" + breakInlineClosers(bundleCSS) + "</style>"
	}
	var scripts string
	if im.Imports == nil {
		// Legacy build: inlined IIFE bundle, no vendored deps.
		scripts = "<script>" + themeSyncJS + "</script>\n" +
			"<script>" + breakInlineClosers(bundleJS) + "</script>"
	} else {
		// ESM build: an import map (bare specifier -> /vendor/<sha>) then the module.
		imJSON, _ := json.Marshal(im)
		scripts = "<script>" + themeSyncJS + "</script>\n" +
			`<script type="importmap">` + breakInlineClosers(string(imJSON)) + "</script>\n" +
			`<script type="module">` + breakInlineClosers(bundleJS) + "</script>"
	}
	htmlOpen := `<html lang="en">`
	if theme != "" {
		htmlOpen = `<html lang="en" data-theme="` + html.EscapeString(theme) + `">`
	}
	return `<!doctype html>
` + htmlOpen + `
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

// previewBridgeEmulatorJS stands in for the production host's bridge: it answers
// the shard SDK's hello/theme.get with the stamped document theme and nodes.get/
// subscribe with stub nodes, so themed and data-wired shards render in the
// headless preview instead of hanging on a host that isn't there. Preview-only —
// EntryHTML (the served doc) never carries it; the real app's host responds with
// workspace data. See shard-sdk.tsx for the client side.
const previewBridgeEmulatorJS = `(function(){
  if(window.__aladinBridgeEmu)return; window.__aladinBridgeEmu=1;
  function node(id){return {id:id,type:"preview",title:"Preview node "+id,
    data:{id:id,note:"preview stub — the live host serves nodes.get from your workspace"}};}
  function reply(r){window.postMessage(r,"*");}
  function theme(){return document.documentElement.getAttribute("data-theme")||"dark";}
  // In-memory shard-KV honoring the revision guard + prefix subscriptions, so
  // stateful shards RUN headless. Scratch by design — the doc-lifetime sandbox
  // (the draft-channel analogue); the live host binds your real published data.
  var store=new Map(); var kvSubs=new Map();
  function entryOf(k){var e=store.get(k);return {key:k,value:e.value,revision:e.revision,deleted:false};}
  function pushKV(k){kvSubs.forEach(function(prefix,channel){
    if(k.indexOf(prefix)===0){var e=store.get(k);
      reply({aladin:"bridge/1",type:"push",channel:channel,
        data:e?entryOf(k):{key:k,value:null,revision:0,deleted:true}});}});}
  function conflict(id,k){var e=store.get(k);
    reply({aladin:"bridge/1",type:"response",id:id,ok:false,error:"conflict on "+k,code:"conflict",
      data:{key:k,currentRevision:e?e.revision:0,currentValue:e?e.value:null,deleted:false}});}
  window.addEventListener("message",function(e){
    var m=e.data; if(!m||m.aladin!=="bridge/1"||m.type!=="request")return;
    var p=m.params||{}; var ids=p.ids||[];
    function ok(data){reply({aladin:"bridge/1",type:"response",id:m.id,ok:true,data:data});}
    if(m.method==="hello"){ok({protocol:"bridge/1",theme:theme(),capabilities:["theme","kv"],
      methods:["hello","theme.get","kv.get","kv.list","kv.set","kv.delete","kv.subscribe","kv.unsubscribe","nodes.get","nodes.subscribe","nodes.unsubscribe"]});}
    else if(m.method==="theme.get"){ok({theme:theme()});}
    else if(m.method==="kv.get"){ok(store.has(p.key)?entryOf(p.key):null);}
    else if(m.method==="kv.list"){var out=[];store.forEach(function(_,k){
      if(k.indexOf(p.prefix||"")===0)out.push(entryOf(k));});ok({entries:out});}
    else if(m.method==="kv.set"){var cur=store.get(p.key);var base=(cur?cur.revision:0);
      if((p.baseRevision||0)!==base){conflict(m.id,p.key);return;}
      store.set(p.key,{value:p.value,revision:base+1});
      ok({revision:base+1});pushKV(p.key);}
    else if(m.method==="kv.delete"){var cur2=store.get(p.key);
      if(cur2&&(p.baseRevision||0)!==cur2.revision){conflict(m.id,p.key);return;}
      store.delete(p.key);ok(true);pushKV(p.key);}
    else if(m.method==="kv.subscribe"){kvSubs.set(p.channel,p.prefix||"");ok(true);
      store.forEach(function(_,k){if(k.indexOf(p.prefix||"")===0)
        reply({aladin:"bridge/1",type:"push",channel:p.channel,data:entryOf(k)});});}
    else if(m.method==="kv.unsubscribe"){kvSubs.delete(p.channel);ok(true);}
    else if(m.method==="nodes.get"){ok(ids.map(node));}
    else if(m.method==="nodes.subscribe"){ok(true);
      ids.forEach(function(id){reply({aladin:"bridge/1",type:"push",channel:p.channel,data:node(id)});});}
    else if(m.method==="nodes.unsubscribe"){ok(true);}
    else{reply({aladin:"bridge/1",type:"response",id:m.id,ok:false,error:"preview emulator: unknown method "+m.method,code:"unknown-method"});}
  });
})();`

// previewRejectionCaptureJS surfaces unhandled promise rejections. CDP's
// Runtime.exceptionThrown does NOT fire for them, so a shard whose data load
// rejects (the most common real failure) looked perfectly healthy to the gate.
// Routing them through console.error puts them in the same accumulator the
// verify pass already reads. Preview-only: served docs don't carry it.
const previewRejectionCaptureJS = `window.addEventListener("unhandledrejection",function(e){
  var r=e&&e.reason; console.error("[unhandledrejection] "+((r&&(r.stack||r.message))||String(r)));});`

// PreviewHTML is EntryHTML for the headless preview renderer. It is the same
// document the serve route emits, plus two preview-only injections: the CSP as a
// <meta http-equiv> tag (page.SetDocumentContent loads HTML with no HTTP headers,
// so the policy must travel inline) and a bridge emulator so data-wired shards
// render without the production host.
func PreviewHTML(title, tokensCSS, bundleCSS, bundleJS, csp string, im ImportMap, theme string) string {
	doc := EntryHTML(title, tokensCSS, bundleCSS, bundleJS, im, theme)
	// Insert immediately after the charset meta: the CSP first (so it's the first
	// thing the parser applies), then the bridge emulator (before the body bundle,
	// so its listener is ready when the shard mounts). EntryHTML always emits this
	// exact charset line.
	head := "<meta http-equiv=\"Content-Security-Policy\" content=\"" + csp + "\">\n" +
		"<script>" + previewRejectionCaptureJS + "</script>\n" +
		"<script>" + breakInlineClosers(previewBridgeEmulatorJS) + "</script>\n"
	return strings.Replace(doc, "<meta charset=\"utf-8\">\n", "<meta charset=\"utf-8\">\n"+head, 1)
}

// LostCredentialHTML is what a BROWSER gets instead of {"error":"Unauthenticated"}.
//
// An initial load with an expired/missing token and a link that drops the token
// both land here. Do not assume a navigation bug from an authentication failure.
func LostCredentialHTML() string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>Signed out of this view</title>` +
		`<meta name="referrer" content="no-referrer"><style>` + TokensCSS +
		`body{padding:32px;font-family:var(--font-sans);color:var(--ink-2);line-height:1.6}` +
		`h1{font-family:var(--font-display);font-size:15px;color:var(--ink);margin:0 0 8px}` +
		`code{font-family:var(--font-mono);font-size:12px;color:var(--ink-3)}</style></head>` +
		`<body><h1>This view could not authenticate</h1>` +
		`<p>The credential in this shard's URL is missing, expired, or no longer valid. ` +
		`Close and reopen the shard to request a fresh credential. If it still fails, ` +
		`restart Aladin and sign in again.</p>` +
		`<p>If this happened after following a link inside the shard, its links must use ` +
		`<code>href="#/section"</code>, not <code>href="/section"</code>.</p></body></html>`
}

// NotBuiltHTML is shown when a page has no built bundle yet.
func NotBuiltHTML(title string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>` +
		html.EscapeString(title) + `</title><style>` + TokensCSS +
		`body{padding:32px;font-family:var(--font-sans)}</style></head>` +
		`<body><p style="color:var(--ink-2)">This page hasn't been built yet. Run build_app.</p></body></html>`
}
