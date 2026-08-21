package docsurface

import (
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
)

// A shard that uses one component from each new family, authored the way an
// agent would. Building this proves the kit compiles TOGETHER with a real
// shard; rendering it proves the components actually work — the distinction the
// authoring instructions insist on ("compiling != working").
const kitShardIndexTSX = `import { createRoot } from "react-dom/client";
import {
  Page, Region, DataTable, MetricRow, Sparkline, KeyValue, ProgressBar,
  AppShell, SearchInput, Select, Checkbox, EmptyState, LoadingState, Toasts,
  Quiz, Flashcards, Timer, Checklist, Stepper, useShardState, useKV, useTheme,
} from "@aladin/kit";

type Row = { symbol: string; qty: number };
const rows: Row[] = [
  { symbol: "AAPL", qty: 10 },
  { symbol: "MSFT", qty: 4 },
];

function App() {
  const theme = useTheme();
  const [note, setNote] = useShardState<string>("notes/scratch", "");
  const log = useKV("expenses/");
  return (
    <AppShell title="Kit smoke" nav={[{ id: "home", label: "Home", to: "#/" }]}>
      <Page>
        <Region anchor="metrics" kind="metric">
          <MetricRow metrics={[{ label: "Positions", value: rows.length, delta: 2 }]} />
        </Region>
        <Region anchor="positions:table" kind="collection">
          <DataTable
            columns={[
              { key: "symbol", label: "Symbol" },
              { key: "qty", label: "Qty", align: "right" },
            ]}
            rows={rows}
            rowKey={(r) => r.symbol}
          />
        </Region>
        <Region anchor="detail">
          <KeyValue items={[{ label: "Theme", value: theme || "dark" }]} />
          <Sparkline points={[1, 3, 2, 5]} />
          <ProgressBar value={40} label="Filled" />
          <SearchInput value={note} onChange={setNote} />
          <Select options={[{ value: "a", label: "A" }]} value="a" onChange={() => {}} />
          <Checkbox checked={false} onChange={() => {}} label="Done" />
          {log.loading ? <LoadingState /> : <EmptyState title="No expenses" />}
        </Region>
        <Region anchor="interactive" kind="control">
          <Quiz
            questions={[
              { id: "q1", prompt: "2+2?", choices: [{ id: "a", text: "4" }, { id: "b", text: "5" }], answerId: "a" },
            ]}
            stateKey="quiz/basics"
          />
          <Flashcards cards={[{ id: "c1", front: "front", back: "back" }]} stateKey="cards/deck" />
          <Timer seconds={60} label="Pomodoro" stateKey="timer/pomodoro" />
          <Checklist items={[{ id: "i1", label: "Ship it" }]} stateKey="checklist/today" />
          <Stepper steps={[{ id: "s1", title: "First", content: "step one" }]} stateKey="stepper/tour" />
        </Region>
        <Toasts />
      </Page>
    </AppShell>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
`

// End-to-end: a shard composed from every new kit family builds and RENDERS,
// with its declared anchors present in the DOM and no uncaught exceptions.
func TestKitComponentsRenderInPreview(t *testing.T) {
	chromeAvailable(t)
	root := t.TempDir()
	store := NewStore(root)
	ctx := testCtx()
	if _, err := store.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatalf("EnsurePageDir: %v", err)
	}
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(kitShardIndexTSX)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// A real build (esbuild + the embedded kit). Vendored deps come from the
	// shared cache/network, so skip rather than fail when unreachable.
	builder := NewBuilder(store, t.TempDir())
	res, err := builder.Build(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Skipf("kit shard build unavailable (needs esm.sh for react): %v", err)
	}
	if !res.OK {
		if strings.Contains(res.Log, "esm.sh") || strings.Contains(res.Log, "dial tcp") {
			t.Skipf("kit shard build needs network: %s", res.Log)
		}
		t.Fatalf("kit shard did not build:\n%s", res.Log)
	}

	m := NewPreviewSessions(store, builder, PreviewOptions{}).(*PreviewSessions)
	t.Cleanup(func() { _ = m.CloseAll(ctx) })

	st, err := m.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{Theme: "light"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !st.Mounted {
		t.Fatalf("kit shard did not mount: %+v", st)
	}
	if len(st.Exceptions) > 0 {
		t.Fatalf("kit shard threw: %v", st.Exceptions)
	}

	// Every declared region is really in the DOM (what the publish gate checks).
	counts, err := m.CheckAnchors(ctx, "p1", []string{"metrics", "positions:table", "detail", "interactive"})
	if err != nil {
		t.Fatalf("CheckAnchors: %v", err)
	}
	for anchor, n := range counts {
		if n == 0 {
			t.Errorf("anchor %q rendered 0 times", anchor)
		}
	}

	// The stateful components reached the (emulated) KV store rather than
	// throwing, and the table stamped its row keys.
	snap, err := m.Snapshot(ctx, "p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	for _, want := range []string{"AAPL", "Pomodoro", "Ship it"} {
		if !strings.Contains(snap.Outline, want) {
			t.Errorf("rendered outline missing %q:\n%s", want, snap.Outline)
		}
	}
	time.Sleep(200 * time.Millisecond)
	errs, _ := m.ConsoleErrors(ctx, "p1")
	if len(errs) > 0 {
		t.Errorf("kit shard logged console errors: %v", errs)
	}
}

// A shard whose nav is written the way agents actually write it — bare paths
// ("/returns"), matching the Route paths. Before the fix, AppShell emitted those
// verbatim as <a href="/returns">, which navigates the SERVED frame off
// /content/{id}/?access_token=… and replaces the shard with an auth error. The
// preview loads about:blank, so nothing ever caught it.
const kitPathNavIndexTSX = `import { createRoot } from "react-dom/client";
import { AppShell, Route, Link, Region } from "@aladin/kit";

function App() {
  return (
    <AppShell
      title="Nav"
      nav={[
        { id: "overview", label: "Overview", to: "/" },
        { id: "returns", label: "Returns", to: "/returns" },
        { id: "hashy", label: "Hashy", to: "#/hashy" },
        { id: "bare", label: "Bare", to: "bare" },
      ]}
    >
      <Route path="/">
        <Region anchor="overview">
          <Link to="/returns">go to returns</Link>
        </Region>
      </Route>
      <Route path="/returns">
        <Region anchor="returns">returns body</Region>
      </Route>
    </AppShell>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
`

// Every link the kit emits must be a fragment link, whichever way the shard spelled
// `to` — and EscapingLinks (the publish gate) must agree there is nothing to flag.
func TestAppShellNavEmitsHashLinksOnly(t *testing.T) {
	chromeAvailable(t)
	root := t.TempDir()
	store := NewStore(root)
	ctx := testCtx()
	if _, err := store.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatalf("EnsurePageDir: %v", err)
	}
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(kitPathNavIndexTSX)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	builder := NewBuilder(store, t.TempDir())
	res, err := builder.Build(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Skipf("build unavailable (needs esm.sh for react): %v", err)
	}
	if !res.OK {
		if strings.Contains(res.Log, "esm.sh") || strings.Contains(res.Log, "dial tcp") {
			t.Skipf("build needs network: %s", res.Log)
		}
		t.Fatalf("shard did not build:\n%s", res.Log)
	}

	m := NewPreviewSessions(store, builder, PreviewOptions{}).(*PreviewSessions)
	t.Cleanup(func() { _ = m.CloseAll(ctx) })
	if _, err := m.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	st, err := m.Eval(ctx, "p1", `Array.from(document.querySelectorAll('a[href]')).map(function(a){return a.getAttribute('href')}).join(' ')`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	for _, want := range []string{"#/", "#/returns", "#/hashy", "#/bare"} {
		if !strings.Contains(st.EvalResult, want) {
			t.Errorf("nav href %q missing from %q", want, st.EvalResult)
		}
	}
	if strings.Contains(st.EvalResult, `"/returns"`) || strings.Contains(st.EvalResult, " /returns") {
		t.Errorf("a non-hash href escaped into the DOM: %q", st.EvalResult)
	}

	// The publish gate's own view of the same page.
	links, err := m.EscapingLinks(ctx, "p1")
	if err != nil {
		t.Fatalf("EscapingLinks: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("EscapingLinks = %v, want none", links)
	}

	// And the hash route the nav points at actually resolves (path/hash forms
	// normalize to the same route, so Route still matches).
	nav, err := m.Navigate(ctx, "p1", "/returns")
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if !nav.Mounted {
		t.Fatalf("route did not mount: %+v", nav)
	}
	counts, err := m.CheckAnchors(ctx, "p1", []string{"returns"})
	if err != nil {
		t.Fatalf("CheckAnchors: %v", err)
	}
	if counts["returns"] == 0 {
		t.Error("route /returns did not render its region")
	}
}

// The gate itself: a hand-rolled anchor that bypasses the kit is still caught,
// while fragment links and explicit schemes are left alone.
const kitEscapingLinkIndexTSX = `import { createRoot } from "react-dom/client";
import { Page, Region } from "@aladin/kit";

function App() {
  return (
    <Page>
      <Region anchor="body">
        <a href="/returns">bad: root-relative</a>
        <a href="sections/quiz">bad: relative</a>
        <a href="#/ok">fine: hash route</a>
        <a href="https://example.com">fine: explicit scheme</a>
        <a href="mailto:x@example.com">fine: mailto</a>
      </Region>
    </Page>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
`

func TestEscapingLinksFlagsNonHashHrefs(t *testing.T) {
	chromeAvailable(t)
	store := NewStore(t.TempDir())
	ctx := testCtx()
	if _, err := store.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatalf("EnsurePageDir: %v", err)
	}
	if err := store.WriteFile(ctx, "p1", "index.tsx", []byte(kitEscapingLinkIndexTSX)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	builder := NewBuilder(store, t.TempDir())
	res, err := builder.Build(ctx, "p1", service.ChannelPublished)
	if err != nil {
		t.Skipf("build unavailable (needs esm.sh for react): %v", err)
	}
	if !res.OK {
		if strings.Contains(res.Log, "esm.sh") || strings.Contains(res.Log, "dial tcp") {
			t.Skipf("build needs network: %s", res.Log)
		}
		t.Fatalf("shard did not build:\n%s", res.Log)
	}
	m := NewPreviewSessions(store, builder, PreviewOptions{}).(*PreviewSessions)
	t.Cleanup(func() { _ = m.CloseAll(ctx) })
	if _, err := m.Open(ctx, "p1", service.ChannelPublished, service.PreviewOpenOptions{}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	links, err := m.EscapingLinks(ctx, "p1")
	if err != nil {
		t.Fatalf("EscapingLinks: %v", err)
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
