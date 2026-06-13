package docsurface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aladin/backend_v2/internal/service"
)

// TestBuildChannels proves the draft/published split: draft lands in dist/draft/
// unminified with an inline source map; published lands in dist/ minified with
// none, and drops the marker at the publish-gate path. Network-dependent (esm.sh
// for the vendored react), so it self-skips offline like the other build tests.
func TestBuildChannels(t *testing.T) {
	if testing.Short() || !esmReachable() {
		t.Skip("esm.sh unreachable; skipping network-dependent build test")
	}
	root := t.TempDir()
	st := NewStore(root)
	b := NewBuilder(st, filepath.Join(root, "cache", "esm"))
	ctx := testCtx()

	if _, err := st.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatalf("EnsurePageDir: %v", err)
	}
	if err := st.WriteFile(ctx, "p1", "index.tsx", []byte(counterTSX)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pageRoot := filepath.Join(root, "users", "user-abc", "pages", "p1")

	// --- draft channel -------------------------------------------------------
	draftRes, err := b.Build(ctx, "p1", service.ChannelDraft)
	if err != nil {
		t.Fatalf("draft Build Go error: %v", err)
	}
	if !draftRes.OK {
		t.Fatalf("draft Build OK=false, log:\n%s", draftRes.Log)
	}
	draftJS := readBuilt(t, filepath.Join(pageRoot, "dist", "draft", "bundle.js"))
	if !strings.Contains(draftJS, "sourceMappingURL=data:") {
		t.Error("draft bundle should carry an inline source map")
	}
	for _, p := range []string{"dist/draft/importmap.json", "dist/draft/" + buildMetaName} {
		if _, err := os.Stat(filepath.Join(pageRoot, p)); err != nil {
			t.Errorf("draft output %s missing: %v", p, err)
		}
	}
	if !strings.Contains(draftRes.ServedURL, "?channel=draft") {
		t.Errorf("draft ServedURL = %q, want ?channel=draft", draftRes.ServedURL)
	}
	if draftRes.BuildID == "" {
		t.Error("draft build should report a BuildID")
	}

	// --- published channel ---------------------------------------------------
	pubRes, err := b.Build(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Fatalf("published Build Go error: %v", err)
	}
	if !pubRes.OK {
		t.Fatalf("published Build OK=false, log:\n%s", pubRes.Log)
	}
	pubJS := readBuilt(t, filepath.Join(pageRoot, "dist", "bundle.js"))
	if strings.Contains(pubJS, "sourceMappingURL=data:") {
		t.Error("published bundle should NOT carry an inline source map")
	}
	if len(pubJS) >= len(draftJS) {
		t.Errorf("published bundle (%d B) should be smaller than draft (%d B)", len(pubJS), len(draftJS))
	}
	// publish_app gates on the published marker at this exact path.
	if _, err := os.Stat(filepath.Join(pageRoot, BuildMetaPath)); err != nil {
		t.Errorf("published build marker missing at gate path %s: %v", BuildMetaPath, err)
	}
	// Distinct bytes (minified vs not) ⇒ distinct build ids.
	if pubRes.BuildID == "" || pubRes.BuildID == draftRes.BuildID {
		t.Errorf("published BuildID %q should be non-empty and differ from draft %q", pubRes.BuildID, draftRes.BuildID)
	}
}

func readBuilt(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
