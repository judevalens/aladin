package repo

import (
	"os"
	"strings"
	"testing"
)

func TestFilesystemArtifactStoreDeleteResourceIsIdempotent(t *testing.T) {
	t.Parallel()

	store := NewFilesystemArtifactStore(t.TempDir(), t.TempDir())
	stored, err := store.SaveResource("file", "report.txt", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("SaveResource error: %v", err)
	}
	path, err := store.ResourcePath(stored.StorageKey)
	if err != nil {
		t.Fatalf("ResourcePath error: %v", err)
	}

	if err := store.DeleteResource(stored.StorageKey); err != nil {
		t.Fatalf("DeleteResource error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("os.Stat error = %v, want missing compensated resource", err)
	}
	if err := store.DeleteResource(stored.StorageKey); err != nil {
		t.Fatalf("second DeleteResource error: %v", err)
	}
}
