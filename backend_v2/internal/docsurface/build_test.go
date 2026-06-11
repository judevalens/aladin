package docsurface

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

const counterTSX = `import { useState } from "react";
import { createRoot } from "react-dom/client";

function App() {
  const [n, setN] = useState(0);
  return <div><h1>Counter</h1><button onClick={() => setN(n + 1)}>{n}</button></div>;
}
createRoot(document.getElementById("root")!).render(<App />);
`

// esmReachable probes esm.sh so the network-dependent build test self-skips offline.
func esmReachable() bool {
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get("https://esm.sh/react@18.3.1")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func TestBuildReactPage(t *testing.T) {
	if testing.Short() || !esmReachable() {
		t.Skip("esm.sh unreachable; skipping network-dependent build test")
	}
	root := t.TempDir()
	st := NewStore(root)
	b := NewBuilder(st, filepath.Join(root, "cache", "esm"))
	ctx := testCtx()

	_, _ = st.EnsurePageDir(ctx, "p1")
	if err := st.WriteFile(ctx, "p1", "index.tsx", []byte(counterTSX)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := b.Build(ctx, "p1")
	if err != nil {
		t.Fatalf("Build returned Go error: %v", err)
	}
	if !res.OK {
		t.Fatalf("Build OK=false, log:\n%s", res.Log)
	}
	bundle := filepath.Join(root, "users", "user-abc", "pages", "p1", "dist", "bundle.js")
	data, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatalf("expected dist/bundle.js: %v", err)
	}
	if len(data) < 10_000 {
		t.Fatalf("bundle suspiciously small (%d bytes) — React not inlined?", len(data))
	}
	// React must be inlined, not left as a bare/external import.
	if strings.Contains(string(data), `from"react"`) || strings.Contains(string(data), `from "react"`) {
		t.Fatalf("bundle still imports react externally — not self-contained")
	}
	if res.ServedURL != "/content/p1/" {
		t.Fatalf("ServedURL = %q", res.ServedURL)
	}
	// A successful build drops the publish-gate marker.
	metaPath := filepath.Join(root, "users", "user-abc", "pages", "p1", BuildMetaPath)
	mb, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("expected build marker %s: %v", BuildMetaPath, err)
	}
	var meta buildMeta
	if err := json.Unmarshal(mb, &meta); err != nil {
		t.Fatalf("build marker is not valid json: %v", err)
	}
	if meta.PageID != "p1" || meta.BuiltAt == "" || meta.BundleBytes <= 0 {
		t.Fatalf("build marker fields wrong: %#v", meta)
	}
}

func TestBuildSyntaxErrorIsSoftFail(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	b := NewBuilder(st, filepath.Join(root, "cache", "esm"))
	ctx := testCtx()
	_, _ = st.EnsurePageDir(ctx, "p1")
	// Broken TSX — should NOT need the network to fail parsing.
	if err := st.WriteFile(ctx, "p1", "index.tsx", []byte("const x = (((;")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := b.Build(ctx, "p1")
	if err != nil {
		t.Fatalf("Build returned Go error (should be soft fail): %v", err)
	}
	if res.OK {
		t.Fatal("Build OK=true for broken source, want OK=false")
	}
	if strings.TrimSpace(res.Log) == "" {
		t.Fatal("expected a non-empty error log for the agent")
	}
}

func TestRequireAllowedCDN(t *testing.T) {
	ok := []string{"https://esm.sh/react@18.3.1", "https://esm.sh/react-dom@18/client", "https://esm.sh/d3"}
	for _, u := range ok {
		if err := requireAllowedCDN(u); err != nil {
			t.Errorf("requireAllowedCDN(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"http://esm.sh/x",           // not https
		"https://evil.com/x",        // wrong host
		"https://169.254.169.254/",  // SSRF: cloud metadata
		"https://localhost:8000/x",  // SSRF: internal
		"https://esm.sh.evil.com/x", // host suffix trick
		"file:///etc/passwd",        // scheme
		"https://esm.sh@evil.com/x", // userinfo trick
	}
	for _, u := range bad {
		if err := requireAllowedCDN(u); err == nil {
			t.Errorf("requireAllowedCDN(%q) = nil, want error", u)
		}
	}
}

func TestValidBareSpec(t *testing.T) {
	ok := []string{"react", "react-dom/client", "@scope/pkg", "@scope/pkg/sub", "lodash.merge", "d3@7"}
	for _, s := range ok {
		if !validBareSpec(s) {
			t.Errorf("validBareSpec(%q) = false, want true", s)
		}
	}
	bad := []string{"", "../etc", "react/../../x", "a b", "a;rm -rf", "https://evil"}
	for _, s := range bad {
		if validBareSpec(s) {
			t.Errorf("validBareSpec(%q) = true, want false", s)
		}
	}
}

func TestBuildMissingEntry(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	b := NewBuilder(st, filepath.Join(root, "cache", "esm"))
	ctx := testCtx()
	_, _ = st.EnsurePageDir(ctx, "p1")
	res, err := b.Build(ctx, "p1")
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if res.OK || !strings.Contains(res.Log, "index.tsx") {
		t.Fatalf("want soft fail mentioning index.tsx, got OK=%v log=%q", res.OK, res.Log)
	}
	// A failed build must NOT leave a publish-gate marker.
	if _, err := os.Stat(filepath.Join(root, "users", "user-abc", "pages", "p1", BuildMetaPath)); err == nil {
		t.Fatal("build marker should be absent after a failed build")
	}
	_ = service.BuildResult{}
}
