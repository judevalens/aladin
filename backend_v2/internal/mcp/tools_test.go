package mcpserver

import (
	"context"
	"errors"
	"io"
	"testing"

	"aladin/backend_v2/internal/service"
)

func TestPageToolsEnforceScopes(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		getResult: service.ArtifactResponse{ID: "page-1", Type: "page", Title: "Page", Content: "body", Metadata: map[string]any{}},
	}
	tools := toolServer{artifacts: artifacts}
	readOnlyCtx := contextWithScopes(service.ScopeArtifactsRead)

	if _, _, err := tools.getPage(readOnlyCtx, nil, getPageInput{ID: "page-1"}); err != nil {
		t.Fatalf("getPage with read scope error: %v", err)
	}
	if _, _, err := tools.createPage(readOnlyCtx, nil, createPageInput{Title: "Draft", Content: "body"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("createPage with read scope error = %v, want forbidden", err)
	}
	if _, _, err := tools.updatePage(readOnlyCtx, nil, updatePageInput{ID: "page-1", Title: stringPtr("Next")}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("updatePage with read scope error = %v, want forbidden", err)
	}
}

// During the M5 BlockNote storage migration the MCP write tools are off so
// that they can't silently drop markdown content. M6 re-enables them on top
// of the converter sidecar.
func TestPageWriteToolsDisabledDuringM5Migration(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{}
	tools := toolServer{artifacts: artifacts}
	writeCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)

	_, _, err := tools.createPage(writeCtx, nil, createPageInput{Title: "Created", Content: "body"})
	if err == nil {
		t.Fatal("createPage expected migration error, got nil")
	}
	var requestErr service.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("createPage error = %T, want service.BadRequest", err)
	}
	if artifacts.createPayload.Type != "" {
		t.Fatalf("artifact service should not be reached; got payload %#v", artifacts.createPayload)
	}

	_, _, err = tools.updatePage(writeCtx, nil, updatePageInput{ID: "page-1", Title: stringPtr("Next")})
	if err == nil {
		t.Fatal("updatePage expected migration error, got nil")
	}
	if !errors.As(err, &requestErr) {
		t.Fatalf("updatePage error = %T, want service.BadRequest", err)
	}
}

func TestPageToolsRejectNonPageArtifacts(t *testing.T) {
	t.Parallel()

	tools := toolServer{artifacts: &fakeArtifactService{
		getResult: service.ArtifactResponse{ID: "link-1", Type: "link", Title: "Link", Metadata: map[string]any{}},
	}}
	readCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)

	if _, _, err := tools.getPage(readCtx, nil, getPageInput{ID: "link-1"}); err == nil {
		t.Fatal("getPage non-page error = nil, want error")
	}
	if _, _, err := tools.updatePage(readCtx, nil, updatePageInput{ID: "link-1", Title: stringPtr("Next")}); err == nil {
		t.Fatal("updatePage non-page error = nil, want error")
	}
}

func TestSearchPagesValidatesAndClampsLimit(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		searchResults: []service.ArtifactResponse{
			{ID: "page-1", Type: "page", Title: "Match", Metadata: map[string]any{}},
			{ID: "link-1", Type: "link", Title: "Skip", Metadata: map[string]any{}},
		},
	}
	tools := toolServer{artifacts: artifacts}
	readCtx := contextWithScopes(service.ScopeArtifactsRead)

	if _, _, err := tools.searchPages(readCtx, nil, searchPagesInput{Query: "   "}); err == nil {
		t.Fatal("searchPages empty query error = nil, want error")
	}
	_, out, err := tools.searchPages(readCtx, nil, searchPagesInput{Query: "agent", Limit: 500})
	if err != nil {
		t.Fatalf("searchPages error: %v", err)
	}
	if artifacts.searchParams == nil || artifacts.searchParams.Limit != 50 {
		t.Fatalf("search params = %#v, want limit clamped to 50", artifacts.searchParams)
	}
	if len(out.Pages) != 1 || out.Pages[0].ID != "page-1" {
		t.Fatalf("pages = %#v, want only page results", out.Pages)
	}
}

func testPrincipal(scopes ...string) service.Principal {
	return service.Principal{
		UserID:    "user-1",
		ActorType: service.ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Email:     "user@example.com",
		Scopes:    scopes,
	}
}

func contextWithScopes(scopes ...string) context.Context {
	return service.WithPrincipal(context.Background(), testPrincipal(scopes...))
}

func stringPtr(value string) *string {
	return &value
}

type fakeArtifactService struct {
	list          []service.ArtifactResponse
	searchResults []service.ArtifactResponse
	browserTree   []service.BrowserTreeNode
	folders       []service.FolderNode
	getResult     service.ArtifactResponse
	createResult  service.ArtifactCreateResponse
	updateResult  service.ArtifactResponse
	createPayload service.ArtifactPayload
	updatePatch   service.ArtifactPatch
	searchParams  *service.PageSearchParams
	err           error
}

func (f *fakeArtifactService) List(ctx context.Context, _ service.ArtifactListParams) ([]service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.list, f.err
}

func (f *fakeArtifactService) SearchPages(ctx context.Context, params service.PageSearchParams) ([]service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	copyParams := params
	f.searchParams = &copyParams
	return f.searchResults, f.err
}

func (f *fakeArtifactService) BrowserTree(ctx context.Context) ([]service.BrowserTreeNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.browserTree, f.err
}

func (f *fakeArtifactService) DeleteBrowserNode(context.Context, string) error {
	return nil
}

func (f *fakeArtifactService) Get(ctx context.Context, _ string) (service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return service.ArtifactResponse{}, err
	}
	return f.getResult, f.err
}

func (f *fakeArtifactService) Create(ctx context.Context, payload service.ArtifactPayload) (service.ArtifactCreateResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return service.ArtifactCreateResponse{}, err
	}
	f.createPayload = payload
	if f.createResult.Artifact.ID != "" {
		return f.createResult, f.err
	}
	return service.ArtifactCreateResponse{
		Artifact: service.ArtifactResponse{
			ID:       "page-created",
			Type:     payload.Type,
			Title:    payload.Title,
			FolderID: payload.FolderID,
			Metadata: payload.Metadata,
		},
	}, f.err
}

func (f *fakeArtifactService) Update(ctx context.Context, _ string, patch service.ArtifactPatch) (service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return service.ArtifactResponse{}, err
	}
	f.updatePatch = patch
	if f.updateResult.ID != "" {
		return f.updateResult, f.err
	}
	return f.getResult, f.err
}

func (f *fakeArtifactService) Delete(context.Context, string) error {
	return service.ErrForbidden
}

func (f *fakeArtifactService) Upload(context.Context, service.ArtifactUploadInput, io.Reader) (service.ArtifactResponse, error) {
	return service.ArtifactResponse{}, service.ErrForbidden
}

func (f *fakeArtifactService) Resource(context.Context, string) (service.ArtifactResource, error) {
	return service.ArtifactResource{}, service.ErrForbidden
}

func (f *fakeArtifactService) ListFolders(ctx context.Context, _ *string) ([]service.FolderNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.folders, f.err
}

func (f *fakeArtifactService) FolderTree(context.Context) ([]service.FolderTreeNode, error) {
	return nil, nil
}

func (f *fakeArtifactService) CreateFolder(context.Context, string, *string) (service.FolderNode, error) {
	return service.FolderNode{}, service.ErrForbidden
}

func (f *fakeArtifactService) UpdateFolder(context.Context, string, service.FolderPatch) (service.FolderNode, error) {
	return service.FolderNode{}, service.ErrForbidden
}

func (f *fakeArtifactService) GetFolder(context.Context, string) (service.FolderNode, error) {
	return service.FolderNode{}, service.ErrNotFound
}

func (f *fakeArtifactService) FolderBreadcrumbs(context.Context, string) ([]service.BreadcrumbItem, error) {
	return nil, nil
}

func (f *fakeArtifactService) CreateBrowserNode(context.Context, service.BrowserNodeCreateInput) (service.BrowserNodeCreateResponse, error) {
	return service.BrowserNodeCreateResponse{}, service.ErrForbidden
}
