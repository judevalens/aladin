package service

import (
	"errors"
	"testing"
)

func TestPageServiceGetLoadsPageDocument(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {
				ID:        "artifact-1",
				Type:      "page",
				Title:     "Memo",
				Content:   "stale",
				Revision:  3,
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
		pageContentByID: map[string]string{"artifact-1": "# Hello"},
	}
	svc := NewPageService(repo)

	page, err := svc.Get(testPrincipalContext(), "artifact-1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if page.Content != "# Hello" {
		t.Fatalf("content = %q, want # Hello", page.Content)
	}
	if page.Revision != 3 {
		t.Fatalf("revision = %d, want 3", page.Revision)
	}
}

func TestPageServiceSavePersistsMarkdown(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {
				ID:        "artifact-1",
				Type:      "page",
				Title:     "Memo",
				Content:   "",
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
	}
	svc := NewPageService(repo)

	page, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{Content: "", Revision: 1})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if page.Content != "" {
		t.Fatalf("content = %q, want empty markdown", page.Content)
	}
	if repo.pageContentByID["artifact-1"] != "" {
		t.Fatalf("stored markdown = %q, want empty markdown", repo.pageContentByID["artifact-1"])
	}
}

func TestPageServiceSaveRejectsStaleRevision(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {
				ID:       "artifact-1",
				Type:     "page",
				Title:    "Memo",
				Revision: 3,
			},
		},
	}
	svc := NewPageService(repo)

	_, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{
		Content:  "stale",
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
				Content:   "# Hello",
				Revision:  1,
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
	}
	svc := NewPageService(repo)
	ctx := testIntegrationPrincipalContext(ScopeArtifactsRead)

	if _, err := svc.Get(ctx, "artifact-1"); err != nil {
		t.Fatalf("Get read-only error = %v, want nil", err)
	}
	if _, err := svc.Save(ctx, "artifact-1", PageSaveInput{Content: "changed", Revision: 2}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Save read-only error = %v, want ErrForbidden", err)
	}
}
