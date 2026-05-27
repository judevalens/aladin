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

func TestPageServiceSavePersistsBlocks(t *testing.T) {
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
	}
	svc := NewPageService(repo)

	blocks := json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"saved"}],"children":[]}]`)
	page, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{Blocks: blocks, Revision: 1})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if string(page.Blocks) != string(blocks) {
		t.Fatalf("returned blocks = %s, want %s", string(page.Blocks), string(blocks))
	}
	stored := repo.pagesByID["artifact-1"]
	if stored == nil || string(stored.blocks) != string(blocks) {
		t.Fatalf("stored = %v, want blocks round-tripped", stored)
	}
	if stored.searchText != "saved" {
		t.Fatalf("search_text = %q, want %q", stored.searchText, "saved")
	}
}

func TestPageServiceSaveRejectsNonArrayBlocks(t *testing.T) {
	t.Parallel()
	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {ID: "artifact-1", Type: "page", Title: "Memo"},
		},
	}
	svc := NewPageService(repo)
	_, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{
		Blocks:   json.RawMessage(`{"not":"array"}`),
		Revision: 1,
	})
	var requestErr BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Save error = %v, want BadRequest", err)
	}
}

func TestPageServiceSaveRejectsStaleRevision(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {ID: "artifact-1", Type: "page", Title: "Memo"},
		},
		pagesByID: map[string]*fakePageStore{
			"artifact-1": {blocks: json.RawMessage(`[]`), revision: 3},
		},
	}
	svc := NewPageService(repo)

	_, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{
		Blocks:   json.RawMessage(`[]`),
		Revision: 3,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Save error = %v, want ErrConflict", err)
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
