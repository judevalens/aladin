package page

import (
	"aladin/backend_v2/internal/artifact"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
)

type fakePageRepository struct {
	artifactByID map[string]artifact.ArtifactResponse
	pagesByID    map[string]*fakePageStore
}

type fakePageStore struct {
	blocks     json.RawMessage
	revision   int64
	searchText string
}

func (f *fakePageRepository) GetArtifact(_ context.Context, id string) (artifact.ArtifactResponse, error) {
	record, ok := f.artifactByID[id]
	if !ok {
		return artifact.ArtifactResponse{}, apperror.ErrNotFound
	}
	if stored := f.pagesByID[id]; stored != nil {
		record.Blocks = stored.blocks
		record.Revision = stored.revision
	}
	return record, nil
}

func (*fakePageRepository) SavePageBlocks(context.Context, string, json.RawMessage, string, int64) (int64, error) {
	return 0, nil
}

func (*fakePageRepository) PageBlockAttribution(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (*fakePageRepository) PageEditHistory(context.Context, string) ([]PageEditEntry, error) {
	return nil, nil
}

func (*fakePageRepository) PageEditDiff(context.Context, string) (PageDiff, error) {
	return PageDiff{}, nil
}

func testPrincipalContext() context.Context {
	return testIntegrationPrincipalContext(auth.ScopeArtifactsRead, auth.ScopeArtifactsWrite)
}

func testIntegrationPrincipalContext(scopes ...string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "user-1", ActorType: auth.ActorTypeIntegrationToken, ActorID: "token-1", Scopes: scopes,
	})
}

func TestPageServiceGetLoadsPageBlocks(t *testing.T) {
	t.Parallel()

	blocks := json.RawMessage(`[{"id":"a","type":"heading","content":[{"type":"text","text":"Hello"}],"children":[]}]`)
	repo := &fakePageRepository{
		artifactByID: map[string]artifact.ArtifactResponse{
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
	svc := NewService(repo)

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
	repo := &fakePageRepository{
		artifactByID: map[string]artifact.ArtifactResponse{
			"artifact-1": {ID: "artifact-1", Type: "page", Title: "Memo"},
		},
	}
	svc := NewService(repo)

	_, err := svc.Save(testPrincipalContext(), "artifact-1", PageSaveInput{
		Blocks:   json.RawMessage(`[{"id":"a","type":"paragraph"}]`),
		Revision: 1,
	})
	var requestErr apperror.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Save error = %v, want BadRequest (collab guard)", err)
	}
	if repo.pagesByID["artifact-1"] != nil {
		t.Fatal("Save must not persist blocks")
	}
}

func TestPageServiceRejectsNonPageArtifacts(t *testing.T) {
	t.Parallel()

	repo := &fakePageRepository{
		artifactByID: map[string]artifact.ArtifactResponse{
			"artifact-1": {
				ID:        "artifact-1",
				Type:      "link",
				Title:     "Saved",
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
	}
	svc := NewService(repo)

	if _, err := svc.Get(testPrincipalContext(), "artifact-1"); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestPageServiceReadOnlyTokenCannotSave(t *testing.T) {
	t.Parallel()

	repo := &fakePageRepository{
		artifactByID: map[string]artifact.ArtifactResponse{
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
	svc := NewService(repo)
	ctx := testIntegrationPrincipalContext(auth.ScopeArtifactsRead)

	if _, err := svc.Get(ctx, "artifact-1"); err != nil {
		t.Fatalf("Get read-only error = %v, want nil", err)
	}
	if _, err := svc.Save(ctx, "artifact-1", PageSaveInput{Blocks: json.RawMessage(`[]`), Revision: 2}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("Save read-only error = %v, want ErrForbidden", err)
	}
}
