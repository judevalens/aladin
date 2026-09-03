package mcpserver

import (
	"aladin/backend_v2/internal/artifact"
	"context"
	"strings"
	"testing"

	"aladin/backend_v2/internal/docsurface"
	docverification "aladin/backend_v2/internal/docsurface/verification"
	"aladin/backend_v2/internal/service"
)

const appArtifactType = "app"

// fakeArtifacts embeds the interface; Get reports an "app" so requireApp passes,
// Update is a no-op. Other methods would panic if called.
type fakeArtifacts struct {
	artifact.ArtifactService
}

func (fakeArtifacts) Get(_ context.Context, id string) (artifact.ArtifactResponse, error) {
	return artifact.ArtifactResponse{ID: id, Type: appArtifactType}, nil
}
func (fakeArtifacts) Update(_ context.Context, id string, _ artifact.ArtifactPatch) (artifact.ArtifactResponse, error) {
	return artifact.ArtifactResponse{ID: id, Type: appArtifactType}, nil
}

// fakeBuild embeds the interface and records which channels Build was called for.
type fakeBuild struct {
	service.ShardBuildService
	built []service.BuildChannel
	fail  bool
}

func (f *fakeBuild) Build(_ context.Context, _ string, ch service.BuildChannel) (service.BuildResult, error) {
	f.built = append(f.built, ch)
	if f.fail {
		return service.BuildResult{OK: false, Log: "syntax error in index.tsx"}, nil
	}
	return service.BuildResult{OK: true, BuildID: "deadbeef"}, nil
}

// fakeStore embeds the interface (unused methods would panic) and serves a fixed
// set of files for ReadFile — enough to exercise the verification pass.
type fakeStore struct {
	service.DocSurfaceStore
	files map[string]string
}

func (f fakeStore) ReadFile(_ context.Context, _, relPath string) ([]byte, error) {
	if v, ok := f.files[relPath]; ok {
		return []byte(v), nil
	}
	return nil, service.ErrNotFound
}

// fakePreview embeds the interface and overrides Open/Navigate plus the two
// verification reads. Open always "lands" on "#/"; Navigate looks the route's
// state up in `states`; anchors reports what the DOM would contain per route.
type fakePreview struct {
	service.PreviewService
	openErr       error
	states        map[string]service.PreviewState // route -> state ("#/" is the Open landing)
	anchors       map[string][]string             // route -> anchor ids present in the DOM
	consoleErrors map[string][]string             // route -> console.error lines
	escapingLinks map[string][]string             // route -> non-hash hrefs in the DOM
	route         string                          // current route (tracked across Navigate)
}

func (f *fakePreview) Open(_ context.Context, _ string, _ service.BuildChannel, _ service.PreviewOpenOptions) (service.PreviewState, error) {
	if f.openErr != nil {
		return service.PreviewState{}, f.openErr
	}
	f.route = "#/"
	st := f.states["#/"]
	st.URL = "about:blank#/"
	return st, nil
}

func (f *fakePreview) Navigate(_ context.Context, _, route string) (service.PreviewState, error) {
	f.route = route
	return f.states[route], nil
}

func (f *fakePreview) CheckAnchors(_ context.Context, _ string, ids []string) (map[string]int, error) {
	present := map[string]bool{}
	for _, id := range f.anchors[f.route] {
		present[id] = true
	}
	out := map[string]int{}
	for _, id := range ids {
		if present[id] {
			out[id] = 1
		} else {
			out[id] = 0
		}
	}
	return out, nil
}

func (f *fakePreview) ConsoleErrors(_ context.Context, _ string) ([]string, error) {
	return f.consoleErrors[f.route], nil
}

func (f *fakePreview) EscapingLinks(_ context.Context, _ string) ([]string, error) {
	return f.escapingLinks[f.route], nil
}

func mounted() service.PreviewState    { return service.PreviewState{Mounted: true} }
func notMounted() service.PreviewState { return service.PreviewState{Mounted: false} }

const twoRouteManifest = `{"version":1,"anchors":[
  {"id":"home","route":"#/","meaning":"x"},
  {"id":"a","route":"#/a","meaning":"x"},
  {"id":"a2","route":"#/a","meaning":"dup route, dedup"},
  {"id":"b","route":"#/b","meaning":"x"}
]}`

// allAnchorsPresent mirrors twoRouteManifest — every declared anchor renders.
func allAnchorsPresent() map[string][]string {
	return map[string][]string{"#/": {"home"}, "#/a": {"a", "a2"}, "#/b": {"b"}}
}

func healthyPreview() *fakePreview {
	return &fakePreview{
		states:  map[string]service.PreviewState{"#/": mounted(), "#/a": mounted(), "#/b": mounted()},
		anchors: allAnchorsPresent(),
	}
}

func TestVerifyApp(t *testing.T) {
	ctx := context.Background()

	t.Run("all routes mount with their anchors → ok", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: healthyPreview(),
		}
		report, err := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if err != nil || !report.OK || docverification.FailureSummary(report) != "" {
			t.Fatalf("want ok report, got %+v err=%v", report, err)
		}
		if len(report.Routes) != 3 {
			t.Fatalf("want 3 routes checked (deduped), got %d", len(report.Routes))
		}
	})

	t.Run("a route does not mount → failure names it", func(t *testing.T) {
		p := healthyPreview()
		p.states["#/a"] = notMounted()
		ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}, preview: p}
		report, err := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if err != nil {
			t.Fatalf("verifyApp: %v", err)
		}
		if msg := docverification.FailureSummary(report); !strings.Contains(msg, "#/a") || !strings.Contains(msg, "did not mount") {
			t.Fatalf("want failure naming #/a, got %q", msg)
		}
	})

	t.Run("a route throws → failure names the exception", func(t *testing.T) {
		p := healthyPreview()
		p.states["#/b"] = service.PreviewState{Mounted: true, Exceptions: []string{"TypeError: boom\n  at x"}}
		ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}, preview: p}
		report, _ := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if msg := docverification.FailureSummary(report); !strings.Contains(msg, "#/b") || !strings.Contains(msg, "TypeError: boom") {
			t.Fatalf("want failure naming #/b's exception, got %q", msg)
		}
	})

	// A route can render perfectly and still be unreachable: a non-hash href
	// navigates the SERVED frame off its ?access_token URL, so the shard is
	// replaced by an auth error the moment the link is clicked. Preview runs from
	// about:blank, so only this check ever sees it.
	t.Run("a link that navigates off the shard → failure names the href", func(t *testing.T) {
		p := healthyPreview()
		p.escapingLinks = map[string][]string{"#/a": {"/returns", "sections/quiz"}}
		ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}, preview: p}
		report, _ := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if report.OK {
			t.Fatalf("an escaping link should fail the pass: %+v", report)
		}
		msg := docverification.FailureSummary(report)
		if !strings.Contains(msg, "#/a") || !strings.Contains(msg, "/returns") || !strings.Contains(msg, "401") {
			t.Fatalf("want failure naming #/a's escaping link, got %q", msg)
		}
	})

	// The check the manifest always promised and never had.
	t.Run("a declared anchor missing from the DOM → failure names it", func(t *testing.T) {
		p := healthyPreview()
		p.anchors["#/a"] = []string{"a"} // "a2" declared but never rendered
		ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}, preview: p}
		report, _ := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if report.OK {
			t.Fatalf("missing anchor should fail the pass")
		}
		if msg := docverification.FailureSummary(report); !strings.Contains(msg, "a2") {
			t.Fatalf("want failure naming the missing anchor, got %q", msg)
		}
	})

	t.Run("console errors are reported but only fail under strict_console", func(t *testing.T) {
		p := healthyPreview()
		p.consoleErrors = map[string][]string{"#/b": {"error: deprecation warning from a vendored lib"}}
		ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}, preview: p}

		lenient, _ := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if !lenient.OK {
			t.Fatalf("console errors must not fail the default pass: %+v", lenient)
		}
		var reported bool
		for _, r := range lenient.Routes {
			if len(r.ConsoleErrors) > 0 {
				reported = true
			}
		}
		if !reported {
			t.Fatalf("console errors should still be REPORTED")
		}

		strict, _ := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, true, nil)
		if strict.OK || !strings.Contains(docverification.FailureSummary(strict), "console error") {
			t.Fatalf("strict_console should fail on console errors: %+v", strict)
		}
	})

	// A corrupt manifest used to silently degrade to "check just the root".
	t.Run("invalid manifest is reported, not silently skipped", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{"anchors.json": `{"version":2,"anchors":[{"route":"#/"}]}`}},
			preview: healthyPreview(),
		}
		report, err := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if err != nil {
			t.Fatalf("verifyApp: %v", err)
		}
		if report.OK || len(report.ManifestProblems) == 0 {
			t.Fatalf("want manifest problems reported, got %+v", report)
		}
	})

	t.Run("renderer unavailable → soft warn, not a failure", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: &fakePreview{openErr: service.BadRequest("renderer unavailable: no chrome")},
		}
		report, err := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if err != nil || report.RendererAvailable || report.Warning == "" {
			t.Fatalf("want soft warn, got %+v err=%v", report, err)
		}
		if docverification.FailureSummary(report) != "" {
			t.Fatalf("unverifiable must not read as failed")
		}
	})

	t.Run("real Open failure (e.g. build) → propagated as error", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: &fakePreview{openErr: service.BadRequest("build failed: syntax error")},
		}
		if _, err := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil); err == nil ||
			!strings.Contains(err.Error(), "build failed") {
			t.Fatalf("want build error propagated, got %v", err)
		}
	})

	t.Run("no manifest → verifies just the root", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{}}, // ReadFile → ErrNotFound
			preview: &fakePreview{states: map[string]service.PreviewState{"#/": mounted()}},
		}
		report, err := docverification.NewVerification(ts.store, ts.preview).Verify(ctx, "p1", service.ChannelPublished, false, nil)
		if err != nil || !report.OK || len(report.Routes) != 1 {
			t.Fatalf("want root-only verification, got %+v err=%v", report, err)
		}
	})
}

func TestPublishGate(t *testing.T) {
	ctx := context.Background()
	files := map[string]string{
		docsurface.BuildMetaPath:    `{"build_id":"x"}`,
		docsurface.ManifestFileName: twoRouteManifest,
	}

	// Publish must build the PUBLISHED channel itself: writes auto-build DRAFT,
	// so trusting an old marker could ship bytes nothing verified.
	t.Run("builds published, verifies, then reconciles draft", func(t *testing.T) {
		fb := &fakeBuild{}
		ts := docToolServer{
			artifacts: fakeArtifacts{},
			store:     fakeStore{files: files},
			preview:   healthyPreview(),
			build:     fb,
		}
		_, out, err := ts.publishApp(ctx, nil, publishAppInput{PageID: "p1"})
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
		if !out.OK || !out.Verified {
			t.Fatalf("want ok+verified publish, got %+v", out)
		}
		var sawPublished, sawDraft bool
		for _, ch := range fb.built {
			sawPublished = sawPublished || ch == service.ChannelPublished
			sawDraft = sawDraft || ch == service.ChannelDraft
		}
		if !sawPublished {
			t.Errorf("publish must build the published channel; built=%v", fb.built)
		}
		if !sawDraft {
			t.Errorf("publish must reconcile draft build-state; built=%v", fb.built)
		}
	})

	t.Run("a failing published build blocks publish", func(t *testing.T) {
		ts := docToolServer{
			artifacts: fakeArtifacts{},
			store:     fakeStore{files: files},
			preview:   healthyPreview(),
			build:     &fakeBuild{fail: true},
		}
		_, _, err := ts.publishApp(ctx, nil, publishAppInput{PageID: "p1"})
		if err == nil || !strings.Contains(err.Error(), "syntax error") {
			t.Fatalf("want the build log surfaced, got %v", err)
		}
	})

	t.Run("a broken route blocks publish", func(t *testing.T) {
		p := healthyPreview()
		p.states["#/b"] = notMounted()
		ts := docToolServer{
			artifacts: fakeArtifacts{},
			store:     fakeStore{files: files},
			preview:   p,
			build:     &fakeBuild{},
		}
		_, _, err := ts.publishApp(ctx, nil, publishAppInput{PageID: "p1"})
		if err == nil || !strings.Contains(err.Error(), "#/b") || !strings.Contains(err.Error(), "publish blocked") {
			t.Fatalf("want publish blocked naming #/b, got %v", err)
		}
	})

	t.Run("a missing declared anchor blocks publish", func(t *testing.T) {
		p := healthyPreview()
		p.anchors["#/"] = nil // "home" declared, never rendered
		ts := docToolServer{
			artifacts: fakeArtifacts{},
			store:     fakeStore{files: files},
			preview:   p,
			build:     &fakeBuild{},
		}
		_, _, err := ts.publishApp(ctx, nil, publishAppInput{PageID: "p1"})
		if err == nil || !strings.Contains(err.Error(), "home") {
			t.Fatalf("want publish blocked naming the missing anchor, got %v", err)
		}
	})
}

func TestManifestAnchorsByRouteDedup(t *testing.T) {
	ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}}
	byRoute, routes := docverification.NewVerification(ts.store, ts.preview).ManifestAnchorsByRoute(context.Background(), "p1")
	if want := "#/,#/a,#/b"; strings.Join(routes, ",") != want {
		t.Fatalf("routes = %v, want %s (distinct, in order)", routes, want)
	}
	if got := strings.Join(byRoute["#/a"], ","); got != "a,a2" {
		t.Fatalf("anchors for #/a = %q, want both declared ids", got)
	}
}
