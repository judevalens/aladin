package mcpserver

import (
	"context"
	"strings"
	"testing"

	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"
)

const (
	historyDir  = docsurface.HistoryDir
	historyKeep = docsurface.HistoryKeep
)

// writableStore is a fakeStore that actually accepts writes/deletes, so the
// overwrite guard and the .history snapshots can be observed.
type writableStore struct {
	service.DocSurfaceStore
	files map[string]string
}

func (s *writableStore) ReadFile(_ context.Context, _, relPath string) ([]byte, error) {
	if v, ok := s.files[relPath]; ok {
		return []byte(v), nil
	}
	return nil, service.ErrNotFound
}

func (s *writableStore) WriteFile(_ context.Context, _, relPath string, data []byte) error {
	s.files[relPath] = string(data)
	return nil
}

func (s *writableStore) DeleteFile(_ context.Context, _, relPath string) error {
	delete(s.files, relPath)
	return nil
}

func (s *writableStore) ListDir(_ context.Context, _, relPath string) ([]service.FileEntry, error) {
	seen := map[string]bool{}
	var out []service.FileEntry
	prefix := strings.TrimSuffix(relPath, "/") + "/"
	for name := range s.files {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			dir := rest[:i]
			if !seen[dir] {
				seen[dir] = true
				out = append(out, service.FileEntry{Name: dir, IsDir: true})
			}
			continue
		}
		out = append(out, service.FileEntry{Name: rest})
	}
	return out, nil
}

func historySnapshots(s *writableStore) []string {
	var out []string
	for name := range s.files {
		if strings.HasPrefix(name, historyDir+"/") {
			out = append(out, name)
		}
	}
	return out
}

// write_file used to clobber silently. Overwriting is now opt-in, and whatever
// it replaces is recoverable.
func TestWriteFileOverwriteGuard(t *testing.T) {
	ctx := context.Background()
	store := &writableStore{files: map[string]string{"index.tsx": "original"}}
	ts := docToolServer{artifacts: fakeArtifacts{}, store: store}
	no := false

	t.Run("writing an existing path without overwrite is refused", func(t *testing.T) {
		_, _, err := ts.writeFile(ctx, nil, writeFileInput{PageID: "p1", Path: "index.tsx", Content: "clobbered", Build: &no})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("want refusal, got %v", err)
		}
		if store.files["index.tsx"] != "original" {
			t.Fatalf("file was modified despite refusal: %q", store.files["index.tsx"])
		}
		if len(historySnapshots(store)) != 0 {
			t.Fatalf("a refused write must not snapshot")
		}
	})

	t.Run("a NEW path writes with no ceremony", func(t *testing.T) {
		if _, _, err := ts.writeFile(ctx, nil, writeFileInput{PageID: "p1", Path: "extra.tsx", Content: "new", Build: &no}); err != nil {
			t.Fatalf("new file write: %v", err)
		}
		if store.files["extra.tsx"] != "new" {
			t.Fatalf("new file not written")
		}
	})

	t.Run("overwrite:true replaces and snapshots the previous bytes", func(t *testing.T) {
		if _, _, err := ts.writeFile(ctx, nil, writeFileInput{
			PageID: "p1", Path: "index.tsx", Content: "replaced", Overwrite: true, Build: &no,
		}); err != nil {
			t.Fatalf("overwrite: %v", err)
		}
		if store.files["index.tsx"] != "replaced" {
			t.Fatalf("overwrite did not apply")
		}
		snaps := historySnapshots(store)
		if len(snaps) != 1 || !strings.HasSuffix(snaps[0], "index.tsx") {
			t.Fatalf("want one snapshot of index.tsx, got %v", snaps)
		}
		if store.files[snaps[0]] != "original" {
			t.Fatalf("snapshot holds %q, want the pre-overwrite bytes", store.files[snaps[0]])
		}
	})

	t.Run("delete snapshots too", func(t *testing.T) {
		before := len(historySnapshots(store))
		if _, _, err := ts.deleteFile(ctx, nil, deleteFileInput{PageID: "p1", Path: "extra.tsx", Build: &no}); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, ok := store.files["extra.tsx"]; ok {
			t.Fatalf("file not deleted")
		}
		if got := len(historySnapshots(store)); got != before+1 {
			t.Fatalf("snapshots = %d, want %d", got, before+1)
		}
	})
}

// History is bounded: it's a safety net, not a repository.
func TestHistoryPrunes(t *testing.T) {
	ctx := context.Background()
	store := &writableStore{files: map[string]string{}}
	ts := docToolServer{artifacts: fakeArtifacts{}, store: store}
	no := false

	for i := 0; i < historyKeep+5; i++ {
		store.files["f.tsx"] = "v"
		if _, _, err := ts.writeFile(ctx, nil, writeFileInput{
			PageID: "p1", Path: "f.tsx", Content: "v", Overwrite: true, Build: &no,
		}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if got := len(historySnapshots(store)); got > historyKeep {
		t.Fatalf("history kept %d snapshots, want <= %d", got, historyKeep)
	}
}
