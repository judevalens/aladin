package docsurface

import (
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

const customShardIndexTSX = `import { createRoot } from "react-dom/client";
import { useShardState } from "@aladin/shard";

function App() {
  const [note, setNote] = useShardState("notes/scratch", "Independent interface");
  return <main className="min-h-screen bg-bg p-8 text-ink">
    <section data-anchor="workspace" data-kind="control" className="rounded-card border border-line bg-panel p-6 shadow-panel">
      <p className="font-mono text-meta text-amber">CUSTOM SHARD</p>
      <h1 className="font-display text-title">{note}</h1>
      <button className="rounded-control bg-amber px-3 py-2 text-small text-bg" onClick={() => setNote("Saved through the SDK")}>Save</button>
    </section>
  </main>;
}

createRoot(document.getElementById("root")!).render(<App />);`

// A real authored shard uses raw React markup, token-backed Tailwind classes and
// only the nonvisual SDK. It must build, mount, expose anchors and persist state.
func TestCustomShardRendersWithTokensAndNonvisualSDK(t *testing.T) {
	chromeAvailable(t)
	store := NewStore(t.TempDir())
	ctx := testCtx()
	if _, err := store.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(customShardIndexTSX)); err != nil {
		t.Fatal(err)
	}
	builder := NewBuilder(store, t.TempDir())
	res, err := builder.Build(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Skipf("build unavailable (needs esm.sh for React): %v", err)
	}
	if !res.OK {
		if strings.Contains(res.Log, "esm.sh") || strings.Contains(res.Log, "dial tcp") {
			t.Skipf("build needs network: %s", res.Log)
		}
		t.Fatalf("custom shard did not build:\n%s", res.Log)
	}

	preview := NewPreviewSessions(store, builder, PreviewOptions{}).(*PreviewSessions)
	t.Cleanup(func() { _ = preview.CloseAll(ctx) })
	state, err := preview.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{Theme: "light"})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Mounted || len(state.Exceptions) > 0 {
		t.Fatalf("custom shard failed to mount: %+v", state)
	}
	anchors, err := preview.CheckAnchors(ctx, "p1", []string{"workspace"})
	if err != nil || anchors["workspace"] != 1 {
		t.Fatalf("workspace anchor = %v, err %v", anchors, err)
	}
	if _, err := preview.Click(ctx, "p1", "button"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	snapshot, err := preview.Snapshot(ctx, "p1")
	if err != nil || !strings.Contains(snapshot.Outline, "Saved through the SDK") {
		t.Fatalf("stateful custom UI did not update: %v\n%s", err, snapshot.Outline)
	}
}

func TestRemovedKitImportFailsWithMigrationMessage(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx := testCtx()
	if _, err := store.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	source := `import { Button } from "@aladin/kit"; console.log(Button);`
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(source)); err != nil {
		t.Fatal(err)
	}
	result, err := NewBuilder(store, t.TempDir()).Build(ctx, "p1", service.ChannelDraft)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !strings.Contains(result.Log, "UI components were removed") || !strings.Contains(result.Log, "@aladin/shard") {
		t.Fatalf("removed kit diagnostic = %q", result.Log)
	}
}

const escapingLinkIndexTSX = `import { createRoot } from "react-dom/client";
function App() {
  return <main data-anchor="body">
    <a href="/returns">bad: root-relative</a>
    <a href="sections/quiz">bad: relative</a>
    <a href="#/ok">fine: hash route</a>
    <a href="https://example.com">fine: explicit scheme</a>
    <a href="mailto:x@example.com">fine: mailto</a>
  </main>;
}
createRoot(document.getElementById("root")!).render(<App />);`

func TestEscapingLinksFlagsNonHashHrefs(t *testing.T) {
	chromeAvailable(t)
	store := NewStore(t.TempDir())
	ctx := testCtx()
	if _, err := store.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(escapingLinkIndexTSX)); err != nil {
		t.Fatal(err)
	}
	builder := NewBuilder(store, t.TempDir())
	res, err := builder.Build(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Skipf("build unavailable (needs esm.sh for React): %v", err)
	}
	if !res.OK {
		if strings.Contains(res.Log, "esm.sh") || strings.Contains(res.Log, "dial tcp") {
			t.Skipf("build needs network: %s", res.Log)
		}
		t.Fatalf("shard did not build:\n%s", res.Log)
	}
	preview := NewPreviewSessions(store, builder, PreviewOptions{}).(*PreviewSessions)
	t.Cleanup(func() { _ = preview.CloseAll(ctx) })
	if _, err := preview.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{}); err != nil {
		t.Fatal(err)
	}
	links, err := preview.EscapingLinks(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(links, ",")
	for _, want := range []string{"/returns", "sections/quiz"} {
		if !strings.Contains(got, want) {
			t.Errorf("EscapingLinks %v did not flag %q", links, want)
		}
	}
	for _, unwanted := range []string{"#/ok", "https://example.com", "mailto:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("EscapingLinks %v wrongly flagged %q", links, unwanted)
		}
	}
}
