package mcpserver

import (
	"context"
	"strings"
	"testing"

	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"
)

// fakeArtifacts embeds the interface; Get reports an "app" so requireApp passes,
// Update is a no-op. Other methods would panic if called.
type fakeArtifacts struct {
	service.ArtifactService
}

func (fakeArtifacts) Get(_ context.Context, id string) (service.ArtifactResponse, error) {
	return service.ArtifactResponse{ID: id, Type: appArtifactType}, nil
}
func (fakeArtifacts) Update(_ context.Context, id string, _ service.ArtifactPatch) (service.ArtifactResponse, error) {
	return service.ArtifactResponse{ID: id, Type: appArtifactType}, nil
}

// fakeBuild embeds the interface and records which channels Build was called for.
type fakeBuild struct {
	service.ShardBuildService
	built []service.BuildChannel
}

func (f *fakeBuild) Build(_ context.Context, _ string, ch service.BuildChannel) (service.BuildResult, error) {
	f.built = append(f.built, ch)
	return service.BuildResult{OK: true, BuildID: "deadbeef"}, nil
}

// fakeStore embeds the interface (unused methods would panic) and serves a fixed
// set of files for ReadFile — enough to exercise manifestRoutes/verifyMount.
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

// fakePreview embeds the interface and overrides only Open/Navigate. Open always
// "lands" on "#/"; Navigate looks the route's state up in `states`.
type fakePreview struct {
	service.PreviewService
	openErr error
	states  map[string]service.PreviewState // route -> state ("#/" is the Open landing)
}

func (f fakePreview) Open(_ context.Context, _ string, _ service.BuildChannel) (service.PreviewState, error) {
	if f.openErr != nil {
		return service.PreviewState{}, f.openErr
	}
	st := f.states["#/"]
	st.URL = "#/"
	return st, nil
}

func (f fakePreview) Navigate(_ context.Context, _, route string) (service.PreviewState, error) {
	return f.states[route], nil
}

func mounted() service.PreviewState    { return service.PreviewState{Mounted: true} }
func notMounted() service.PreviewState { return service.PreviewState{Mounted: false} }

const twoRouteManifest = `{"version":1,"anchors":[
  {"id":"home","route":"#/","meaning":"x"},
  {"id":"a","route":"#/a","meaning":"x"},
  {"id":"a2","route":"#/a","meaning":"dup route, dedup"},
  {"id":"b","route":"#/b","meaning":"x"}
]}`

func TestVerifyMount(t *testing.T) {
	ctx := context.Background()

	t.Run("all routes mount → verified", func(t *testing.T) {
		ts := docToolServer{
			store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: fakePreview{states: map[string]service.PreviewState{
				"#/": mounted(), "#/a": mounted(), "#/b": mounted(),
			}},
		}
		ok, warn, err := ts.verifyMount(ctx, "p1")
		if err != nil || !ok || warn != "" {
			t.Fatalf("want verified, got ok=%v warn=%q err=%v", ok, warn, err)
		}
	})

	t.Run("a route does not mount → hard gate", func(t *testing.T) {
		ts := docToolServer{
			store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: fakePreview{states: map[string]service.PreviewState{
				"#/": mounted(), "#/a": notMounted(), "#/b": mounted(),
			}},
		}
		_, _, err := ts.verifyMount(ctx, "p1")
		if err == nil || !strings.Contains(err.Error(), "#/a") || !strings.Contains(err.Error(), "publish blocked") {
			t.Fatalf("want hard gate naming #/a, got %v", err)
		}
	})

	t.Run("a route throws → hard gate names the exception", func(t *testing.T) {
		throws := service.PreviewState{Mounted: true, Exceptions: []string{"TypeError: boom\n  at x"}}
		ts := docToolServer{
			store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: fakePreview{states: map[string]service.PreviewState{
				"#/": mounted(), "#/a": mounted(), "#/b": throws,
			}},
		}
		_, _, err := ts.verifyMount(ctx, "p1")
		if err == nil || !strings.Contains(err.Error(), "#/b") || !strings.Contains(err.Error(), "TypeError: boom") {
			t.Fatalf("want hard gate naming #/b's exception, got %v", err)
		}
	})

	t.Run("renderer unavailable → soft warn, not an error", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: fakePreview{openErr: service.BadRequest("renderer unavailable: no chrome")},
		}
		ok, warn, err := ts.verifyMount(ctx, "p1")
		if err != nil || ok || warn == "" {
			t.Fatalf("want soft warn (ok=false,warn set,no err), got ok=%v warn=%q err=%v", ok, warn, err)
		}
	})

	t.Run("real Open failure (e.g. build) → propagated as error", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}},
			preview: fakePreview{openErr: service.BadRequest("build failed: syntax error")},
		}
		_, _, err := ts.verifyMount(ctx, "p1")
		if err == nil || !strings.Contains(err.Error(), "build failed") {
			t.Fatalf("want build error propagated, got %v", err)
		}
	})

	t.Run("no manifest → falls back to verifying just the root", func(t *testing.T) {
		ts := docToolServer{
			store:   fakeStore{files: map[string]string{}}, // ReadFile → ErrNotFound
			preview: fakePreview{states: map[string]service.PreviewState{"#/": mounted()}},
		}
		ok, _, err := ts.verifyMount(ctx, "p1")
		if err != nil || !ok {
			t.Fatalf("want verified via root fallback, got ok=%v err=%v", ok, err)
		}
	})
}

// A successful publish must reconcile the DRAFT build-state — otherwise a stale
// 'failed' from mid-authoring keeps the work pane showing "build failed" for a
// shard that is in fact live. We assert publishApp triggers a draft rebuild.
func TestPublishReconcilesDraftBuildState(t *testing.T) {
	fb := &fakeBuild{}
	ts := docToolServer{
		artifacts: fakeArtifacts{},
		store: fakeStore{files: map[string]string{
			docsurface.BuildMetaPath:    `{"build_id":"x"}`, // prior successful build marker
			docsurface.ManifestFileName: twoRouteManifest,
		}},
		preview: fakePreview{states: map[string]service.PreviewState{
			"#/": mounted(), "#/a": mounted(), "#/b": mounted(),
		}},
		build: fb,
	}
	_, out, err := ts.publishApp(context.Background(), nil, publishAppInput{PageID: "p1"})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if !out.OK || !out.Verified {
		t.Fatalf("want ok+verified publish, got %+v", out)
	}
	sawDraft := false
	for _, ch := range fb.built {
		if ch == service.ChannelDraft {
			sawDraft = true
		}
	}
	if !sawDraft {
		t.Fatalf("publish did not rebuild the draft channel; built=%v", fb.built)
	}
}

func TestManifestRoutesDedup(t *testing.T) {
	ts := docToolServer{store: fakeStore{files: map[string]string{"anchors.json": twoRouteManifest}}}
	got := ts.manifestRoutes(context.Background(), "p1")
	want := []string{"#/", "#/a", "#/b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("manifestRoutes = %v, want %v (distinct, in order)", got, want)
	}
}
