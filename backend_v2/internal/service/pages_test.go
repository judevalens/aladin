package service

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPageServiceGetLoadsPageBlocks(t *testing.T) {
	t.Parallel()

	blocks := json.RawMessage(`[{"id":"a","type":"heading","content":[{"type":"text","text":"Hello"}],"children":[]}]`)
	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {
				ID:        "artifact-1",
				Type:      "page",
				Title:     "Memo",
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
		pagesByID: map[string]*fakePageStore{
			"artifact-1": {blocks: blocks, searchText: "Hello", revision: 3},
		},
	}
	svc := NewPageService(repo)

	page, err := svc.Get(testPrincipalContext(), "artifact-1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(page.Blocks) != string(blocks) {
		t.Fatalf("blocks = %s, want %s", string(page.Blocks), string(blocks))
	}
	if page.Revision != 3 {
		t.Fatalf("revision = %d, want 3", page.Revision)
	}
}

func TestPageServiceSaveRefused(t *testing.T) {
	t.Parallel()
	// M8c seam guard: page content is collaborative, so PATCH /api/pages is
	// refused — the editor edits via the Y.Doc, agents via the MCP bridge.
	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {ID: "artifact-1", Type: "page", Title: "Memo"},
		},
	}
	svc := NewPageService(repo)

	_, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{
		Blocks:   json.RawMessage(`[{"id":"a","type":"paragraph"}]`),
		Revision: 1,
	})
	var requestErr BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Save error = %v, want BadRequest (collab guard)", err)
	}
	if repo.pagesByID["artifact-1"] != nil {
		t.Fatal("Save must not persist blocks")
	}
}

func TestPageServiceRejectsNonPageArtifacts(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {
				ID:        "artifact-1",
				Type:      "link",
				Title:     "Saved",
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
	}
	svc := NewPageService(repo)

	if _, err := svc.Get(testPrincipalContext(), "artifact-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestPageServiceReadOnlyTokenCannotSave(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {
				ID:        "artifact-1",
				Type:      "page",
				Title:     "Memo",
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
		pagesByID: map[string]*fakePageStore{
			"artifact-1": {blocks: json.RawMessage(`[]`), revision: 1},
		},
	}
	svc := NewPageService(repo)
	ctx := testIntegrationPrincipalContext(ScopeArtifactsRead)

	if _, err := svc.Get(ctx, "artifact-1"); err != nil {
		t.Fatalf("Get read-only error = %v, want nil", err)
	}
	if _, err := svc.Save(ctx, "artifact-1", PageSaveInput{Blocks: json.RawMessage(`[]`), Revision: 2}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Save read-only error = %v, want ErrForbidden", err)
	}
}
