//go:build integration

// Doc Surface ("app" artifact) end-to-end integration test.
//
// Drives the live MCP server (create_app → write_file → build_app →
// publish_app), then the live API server's content route (GET /content/{id}/...)
// to prove the full pipeline: author → esbuild bundle → serve into the iframe.
//
// Prerequisites (SkipNow if missing):
//   - Postgres at $TEST_DATABASE_URL (sandbox; never the dev DB)
//   - network access to https://esm.sh (build resolves react from the CDN)
//
// Run with:
//
//	cd backend_v2 && TEST_DATABASE_URL=... go test -tags=integration ./internal/mcp/... -v -run TestDocSurface_EndToEnd
package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/api"
	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// docSurfaceHashRouterTSX is a 2-route, hash-routed single-page Doc (HOME on #/,
// SECOND on #/two) with a button that throws — to exercise navigation, snapshot,
// and exception capture through the interactive preview tools.
const docSurfaceHashRouterTSX = `import { useState, useEffect } from "react";
import { createRoot } from "react-dom/client";

function useRoute() {
  const [h, setH] = useState(location.hash || "#/");
  useEffect(() => {
    const on = () => setH(location.hash || "#/");
    addEventListener("hashchange", on);
    return () => removeEventListener("hashchange", on);
  }, []);
  return h;
}

function App() {
  const route = useRoute();
  return (
    <div id="view-root" data-route={route}>
      {route === "#/two" ? <h1>SECOND PAGE</h1> : <h1>HOME PAGE</h1>}
      <button id="boom" onClick={() => { throw new Error("kaboom-e2e"); }}>boom</button>
    </div>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
`

const docSurfaceCounterTSX = `import { useState } from "react";
import { createRoot } from "react-dom/client";

function App() {
  const [n, setN] = useState(0);
  return <div><h1 id="title">Widget</h1><button id="b" onClick={() => setN(n + 1)}>count {n}</button></div>;
}
createRoot(document.getElementById("root")!).render(<App />);
`

func esmReachableTest() bool {
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get("https://esm.sh/react@18.3.1")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func TestDocSurface_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if !esmReachableTest() {
		t.Skip("esm.sh unreachable; skipping Doc Surface build e2e")
	}
	dbURL := dbtest.RequireTestDSN(t)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("postgres not reachable at %s: %v", dbURL, err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}

	// The store + builder read DATA_VOLUME_PATH at construction; point it at a
	// throwaway dir BEFORE building dependencies.
	t.Setenv("DATA_VOLUME_PATH", t.TempDir())

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE artifacts, tree_nodes, page_documents RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	deps := app.NewDependencies(pool)

	adminCtx := service.WithPrincipal(ctx, service.Principal{
		UserID:    defaultAdminUserID,
		ActorType: service.ActorTypeUserSession,
		ActorID:   defaultAdminUserID,
		Email:     "admin@email.com",
	})
	token, err := deps.Auth().CreateIntegrationToken(adminCtx, service.IntegrationTokenInput{
		Name:   "docsurface-e2e",
		Scopes: []string{service.ScopeArtifactsRead, service.ScopeArtifactsWrite},
	})
	if err != nil {
		t.Fatalf("CreateIntegrationToken: %v", err)
	}

	// --- MCP layer: author + build + publish ------------------------------
	// Doc Surface tools need no converter/bridge, so pass nil for both.
	mcpServer := New(":0", deps, deps.PageDocuments(), nil, nil)
	mcpTS := httptest.NewServer(mcpServer.httpServer.Handler)
	defer mcpTS.Close()

	httpClient := &http.Client{
		Transport: bearerRoundTripper{Token: token.Token, Base: http.DefaultTransport},
		Timeout:   60 * time.Second,
	}
	transport := &sdkmcp.StreamableClientTransport{Endpoint: mcpTS.URL + "/mcp", HTTPClient: httpClient}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "docsurface-e2e", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer session.Close()

	created := callTool[map[string]any](t, ctx, session, "create_app", map[string]any{
		"title": "My widget",
		"agent": map[string]any{"id": "claude-code", "name": "Claude Code"},
	})
	pageID, _ := created["id"].(string)
	if pageID == "" {
		t.Fatalf("create_app returned no id: %#v", created)
	}

	// install_lib must reject non-esm.sh URLs (build-time SSRF guard).
	ssrf, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "install_lib",
		Arguments: map[string]any{"page_id": pageID, "name": "react", "url": "http://169.254.169.254/"},
	})
	if err != nil {
		t.Fatalf("install_lib SSRF call: %v", err)
	}
	if !ssrf.IsError {
		t.Fatal("install_lib with an internal URL should be rejected (SSRF guard)")
	}

	callTool[map[string]any](t, ctx, session, "write_file", map[string]any{
		"page_id": pageID,
		"path":    "index.tsx",
		"content": docSurfaceCounterTSX,
	})

	built := callTool[map[string]any](t, ctx, session, "build_app", map[string]any{"page_id": pageID})
	if built["ok"] != true {
		t.Fatalf("build_app failed: %v", built["log"])
	}
	if su, _ := built["served_url"].(string); su != "/content/"+pageID+"/" {
		t.Fatalf("build_app served_url = %q", su)
	}

	pub := callTool[map[string]any](t, ctx, session, "publish_app", map[string]any{
		"page_id": pageID,
		"summary": "A counter widget that increments on click.",
	})
	if pub["ok"] != true {
		t.Fatalf("publish_app failed: %#v", pub)
	}
	// The summary (KG spine) must be on the artifact row now.
	rec, err := deps.Artifacts().Get(adminCtx, pageID)
	if err != nil {
		t.Fatalf("Get after publish: %v", err)
	}
	if rec.Summary == nil || !strings.Contains(*rec.Summary, "counter widget") {
		t.Fatalf("summary not persisted: %#v", rec.Summary)
	}

	// --- publish gate: a never-built page cannot be published --------------
	created2 := callTool[map[string]any](t, ctx, session, "create_app", map[string]any{"title": "Unbuilt"})
	pid2, _ := created2["id"].(string)
	if pid2 == "" {
		t.Fatalf("create_app(2) returned no id: %#v", created2)
	}
	gate, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "publish_app",
		Arguments: map[string]any{"page_id": pid2, "summary": "x"},
	})
	if err != nil {
		t.Fatalf("publish_app gate call: %v", err)
	}
	if !gate.IsError {
		t.Fatal("publish_app on a page that was never built should be rejected (marker gate)")
	}

	// --- interactive preview: walk a 2-route hash-routed page -------------
	// Author + build a hash-routed page on the existing pageID, then drive it.
	callTool[map[string]any](t, ctx, session, "write_file", map[string]any{
		"page_id": pageID, "path": "index.tsx", "content": docSurfaceHashRouterTSX,
	})
	rebuilt := callTool[map[string]any](t, ctx, session, "build_app", map[string]any{"page_id": pageID})
	if rebuilt["ok"] != true {
		t.Fatalf("rebuild hash page failed: %v", rebuilt["log"])
	}

	// The preview assertions need a headless browser; if none is present the tool
	// returns "renderer unavailable" — log and skip just this block, not the test.
	func() {
		openRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "preview_open",
			Arguments: map[string]any{"page_id": pageID},
		})
		if err != nil {
			t.Fatalf("preview_open call: %v", err)
		}
		if openRes.IsError {
			if strings.Contains(contentString(openRes.Content), "renderer unavailable") {
				t.Log("headless renderer unavailable; skipping preview assertions")
				return
			}
			t.Fatalf("preview_open IsError: %s", contentString(openRes.Content))
		}
		if structAny(t, openRes)["mounted"] != true {
			t.Fatalf("preview_open mounted=false: %s", contentString(openRes.Content))
		}

		// Navigate to the second hash route → a different view.
		callTool[map[string]any](t, ctx, session, "preview_navigate", map[string]any{"page_id": pageID, "route": "#/two"})
		snap := callTool[map[string]any](t, ctx, session, "preview_snapshot", map[string]any{"page_id": pageID})
		if outline, _ := snap["outline"].(string); !strings.Contains(outline, "SECOND PAGE") {
			t.Fatalf("route-two snapshot missing SECOND PAGE: %q", outline)
		}

		// Screenshot must come back as an image content block.
		shotRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "preview_screenshot",
			Arguments: map[string]any{"page_id": pageID},
		})
		if err != nil || shotRes.IsError {
			t.Fatalf("preview_screenshot: err=%v content=%s", err, contentString(shotRes.Content))
		}
		if !hasImageContent(shotRes.Content) {
			t.Fatalf("preview_screenshot returned no image content block")
		}

		// Eval returns a scalar.
		if ev := callTool[map[string]any](t, ctx, session, "preview_eval", map[string]any{"page_id": pageID, "expr": "2+5"}); ev["eval_result"] != "7" {
			t.Fatalf("preview_eval 2+5 = %v", ev["eval_result"])
		}

		// Clicking the throwing button surfaces an uncaught exception.
		callTool[map[string]any](t, ctx, session, "preview_click", map[string]any{"page_id": pageID, "selector": "#boom"})
		con := callTool[map[string]any](t, ctx, session, "preview_console", map[string]any{"page_id": pageID})
		if !anyContains(con["exceptions"], "kaboom-e2e") {
			t.Fatalf("preview_console did not capture the click exception: %#v", con["exceptions"])
		}
		callTool[map[string]any](t, ctx, session, "preview_close", map[string]any{"page_id": pageID})
	}()

	// --- API layer: serve the built page ----------------------------------
	apiServer := api.NewWithDependencies(":0", deps)
	apiTS := httptest.NewServer(apiServer.Handler())
	defer apiTS.Close()

	get := func(path, bearer string) (*http.Response, string) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiTS.URL+path, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp, string(body)
	}

	// Entry HTML: authed → 200, self-contained (bundle + tokens INLINED, no
	// sub-resource <link>/<script src> — the opaque-origin sandbox can't load 'self').
	resp, body := get("/content/"+pageID+"/", token.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET entry = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("entry content-type = %q", ct)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'none'") || !strings.Contains(csp, "script-src 'unsafe-inline'") {
		t.Fatalf("CSP header missing/weak: %q", csp)
	}
	if !strings.Contains(body, `id="root"`) {
		t.Fatalf("entry HTML missing #root:\n%.500s", body)
	}
	// Vendored deps: the doc is a small ESM module + an import map -> /vendor.
	if !strings.Contains(body, `<script type="importmap">`) || !strings.Contains(body, `<script type="module">`) {
		t.Fatalf("entry HTML should be ESM (import map + module script):\n%.700s", body)
	}
	if !strings.Contains(body, "/vendor/") {
		t.Fatalf("entry HTML import map should reference /vendor:\n%.700s", body)
	}
	if strings.Contains(body, `src="bundle.js`) || strings.Contains(body, `href="tokens.css`) {
		t.Fatalf("entry HTML must inline tokens/css (no <link>/<script src>):\n%.500s", body)
	}
	if !strings.Contains(body, "--amber") { // design tokens still inlined
		t.Fatalf("entry HTML missing inlined design tokens")
	}

	// The import map references /vendor/<sha>; that route is PUBLIC (no auth) + immutable.
	if mm := regexp.MustCompile(`/vendor/([a-f0-9]{64})`).FindStringSubmatch(body); mm != nil {
		vresp, vbody := get("/vendor/"+mm[1], "") // no token — public route
		if vresp.StatusCode != http.StatusOK {
			t.Fatalf("public GET /vendor/<sha> = %d, want 200", vresp.StatusCode)
		}
		if ac := vresp.Header.Get("Access-Control-Allow-Origin"); ac != "*" {
			t.Fatalf("vendor ACAO = %q, want * (opaque-origin module fetch)", ac)
		}
		if cc := vresp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Fatalf("vendor Cache-Control = %q, want immutable", cc)
		}
		if len(vbody) == 0 {
			t.Fatal("vendor body empty")
		}
	} else {
		t.Fatalf("no /vendor/<sha> in import map:\n%.800s", body)
	}
	if r, _ := get("/vendor/deadbeef", ""); r.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown vendor sha = %d, want 404", r.StatusCode)
	}

	// Desktop iframe path: ?access_token query auth (no Authorization header) →
	// the entry (and its inlined everything) authenticates with no sub-resources.
	resp, qbody := get("/content/"+pageID+"/?access_token="+url.QueryEscape(token.Token), "")
	if resp.StatusCode != http.StatusOK || !strings.Contains(qbody, `id="root"`) {
		t.Fatalf("access_token entry = %d (want 200 with #root)", resp.StatusCode)
	}

	// Unauthenticated → 401 (auth middleware).
	resp, _ = get("/content/"+pageID+"/", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthed GET = %d, want 401", resp.StatusCode)
	}

	// Unknown page → 404.
	resp, _ = get("/content/does-not-exist/", token.Token)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown page GET = %d, want 404", resp.StatusCode)
	}

	// Traversal attempt → not 200.
	resp, _ = get("/content/"+pageID+"/../../etc/passwd", token.Token)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("traversal GET unexpectedly 200")
	}
}

// structAny decodes a tool result's StructuredContent into a generic map.
func structAny(t *testing.T, res *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	out := map[string]any{}
	if res.StructuredContent == nil {
		return out
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	return out
}

// hasImageContent reports whether any content block is a non-empty image.
func hasImageContent(content []sdkmcp.Content) bool {
	for _, c := range content {
		if ic, ok := c.(*sdkmcp.ImageContent); ok && len(ic.Data) > 0 {
			return true
		}
	}
	return false
}

// anyContains reports whether any string in a []any contains sub.
func anyContains(v any, sub string) bool {
	arr, ok := v.([]any)
	if !ok {
		return false
	}
	for _, e := range arr {
		if s, ok := e.(string); ok && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
