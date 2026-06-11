package docsurface

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aladin/backend_v2/internal/service"
)

func testCtx() context.Context {
	return service.WithPrincipal(context.Background(), service.Principal{UserID: "user-abc"})
}

func TestValidateSegment(t *testing.T) {
	bad := []string{"", ".", "..", "a/b", "a\\b", string([]byte{'x', 0, 'y'})}
	for _, s := range bad {
		if err := validateSegment(s); err == nil {
			t.Errorf("validateSegment(%q) = nil, want error", s)
		}
	}
	good := []string{"user-abc", "page_123", "abcDEF09"}
	for _, s := range good {
		if err := validateSegment(s); err != nil {
			t.Errorf("validateSegment(%q) = %v, want nil", s, err)
		}
	}
}

func TestStoreRoundTripAndScoping(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	ctx := testCtx()

	if _, err := st.EnsurePageDir(ctx, "p1"); err != nil {
		t.Fatalf("EnsurePageDir: %v", err)
	}
	if err := st.WriteFile(ctx, "p1", "index.tsx", []byte("export const x = 1;")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := st.WriteFile(ctx, "p1", "components/Btn.tsx", []byte("ok")); err != nil {
		t.Fatalf("WriteFile subdir: %v", err)
	}
	// File landed under users/{uid}/pages/{pid}/.
	want := filepath.Join(root, "users", "user-abc", "pages", "p1", "index.tsx")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected file at %s: %v", want, err)
	}
	b, err := st.ReadFile(ctx, "p1", "index.tsx")
	if err != nil || string(b) != "export const x = 1;" {
		t.Fatalf("ReadFile = %q, %v", b, err)
	}
	ents, err := st.ListDir(ctx, "p1", "")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(ents) != 2 { // index.tsx + components/
		t.Fatalf("ListDir = %d entries, want 2: %+v", len(ents), ents)
	}
}

func TestStoreTraversalRejected(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root)
	ctx := testCtx()
	_, _ = st.EnsurePageDir(ctx, "p1")

	for _, rel := range []string{"../../etc/passwd", "../p2/secret", "/etc/passwd", "a/../../b"} {
		if err := st.WriteFile(ctx, "p1", rel, []byte("x")); err == nil {
			t.Errorf("WriteFile(rel=%q) = nil, want traversal rejection", rel)
		}
	}
	// A pageID with a slash must be rejected outright.
	if err := st.WriteFile(ctx, "../evil", "index.tsx", []byte("x")); err == nil {
		t.Error("WriteFile(pageID=../evil) = nil, want rejection")
	}
}

func TestStoreRequiresPrincipal(t *testing.T) {
	st := NewStore(t.TempDir())
	if _, err := st.EnsurePageDir(context.Background(), "p1"); err == nil {
		t.Error("EnsurePageDir without principal = nil, want unauthenticated error")
	}
}

func TestInstallLibDedup(t *testing.T) {
	st := NewStore(t.TempDir())
	ctx := testCtx()
	_, _ = st.EnsurePageDir(ctx, "p1")

	if _, err := st.InstallLib(ctx, "p1", service.LibEntry{Name: "recharts", URL: "https://esm.sh/recharts@2.10.0"}); err != nil {
		t.Fatalf("InstallLib: %v", err)
	}
	libs, err := st.InstallLib(ctx, "p1", service.LibEntry{Name: "recharts", URL: "https://esm.sh/recharts@2.12.0"})
	if err != nil {
		t.Fatalf("InstallLib 2: %v", err)
	}
	if len(libs) != 1 || libs[0].URL != "https://esm.sh/recharts@2.12.0" {
		t.Fatalf("dedupe failed: %+v", libs)
	}
}
