package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestArtifactServiceCreatePageDefaultsAndTrims(t *testing.T) {
	t.Parallel()

	summary := "  useful memo  "
	repo := &fakeArtifactRepository{}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	rec, err := svc.Create(testPrincipalContext(), ArtifactPayload{
		Type:    "page",
		Content: "  Rivian supply chain memo  ",
		Summary: &summary,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if rec.ID == "" || !strings.HasPrefix(rec.ID, "artifact-") {
		t.Fatalf("id = %q, want artifact-*", rec.ID)
	}
	if len(repo.createdArtifacts) != 1 || repo.createdArtifacts[0].ID != rec.ID {
		t.Fatalf("created = %#v, want one created artifact", repo.createdArtifacts)
	}
}

func TestArtifactServiceRequiresPrincipal(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	if _, err := svc.Create(context.Background(), ArtifactPayload{Type: "page", Title: "Memo"}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Create error = %v, want ErrUnauthenticated", err)
	}
	if _, err := svc.BrowserTree(context.Background()); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("BrowserTree error = %v, want ErrUnauthenticated", err)
	}
}

func TestArtifactServiceReadOnlyTokenCannotWrite(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	ctx := testIntegrationPrincipalContext(ScopeArtifactsRead)

	if _, err := svc.BrowserTree(ctx); err != nil {
		t.Fatalf("BrowserTree read-only error = %v, want nil", err)
	}
	if _, err := svc.Create(ctx, ArtifactPayload{Type: "page", Title: "Memo"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create read-only error = %v, want ErrForbidden", err)
	}
	if err := svc.Delete(ctx, "artifact-1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete read-only error = %v, want ErrForbidden", err)
	}
	if _, err := svc.CreateFolder(ctx, "Folder", nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateFolder read-only error = %v, want ErrForbidden", err)
	}
}

func TestArtifactServiceSearchPagesValidatesAndClamps(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		searchResults: []ArtifactResponse{{ID: "page-1", Type: "page", Title: "Match"}},
	}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	if _, err := svc.SearchPages(testIntegrationPrincipalContext(ScopeArtifactsWrite), PageSearchParams{Query: "memo"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("SearchPages without read scope error = %v, want ErrForbidden", err)
	}
	if _, err := svc.SearchPages(testPrincipalContext(), PageSearchParams{Query: "   "}); err == nil {
		t.Fatal("SearchPages empty query error = nil, want BadRequest")
	}
	results, err := svc.SearchPages(testPrincipalContext(), PageSearchParams{Query: "  memo  ", Limit: 500})
	if err != nil {
		t.Fatalf("SearchPages error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "page-1" {
		t.Fatalf("results = %#v, want fake search result", results)
	}
	if repo.searchParams == nil || repo.searchParams.Query != "memo" || repo.searchParams.Limit != 50 {
		t.Fatalf("search params = %#v, want trimmed query and clamped limit", repo.searchParams)
	}
}

func TestArtifactServiceCreateLinkRequiresSourceURL(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	_, err := svc.Create(testPrincipalContext(), ArtifactPayload{Type: "link", Title: "Saved"})
	if err == nil {
		t.Fatal("Create error = nil, want BadRequest")
	}
	var requestErr BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Create error = %T, want BadRequest", err)
	}
}

func TestArtifactServiceEmptyIDsAreNotFound(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	if _, err := svc.Get(testPrincipalContext(), " "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
	if _, err := svc.Update(testPrincipalContext(), " ", ArtifactPatch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update error = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(testPrincipalContext(), " "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestArtifactServiceUploadCreatesArtifactRecord(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	rec, err := svc.Upload(testPrincipalContext(), ArtifactUploadInput{
		Type:     "file",
		Filename: "memo.txt",
	}, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if rec.Type != "file" {
		t.Fatalf("type = %q, want file", rec.Type)
	}
	if storageKey, _ := rec.Metadata["storageKey"].(string); storageKey == "" {
		t.Fatalf("metadata = %#v, want storageKey", rec.Metadata)
	}
}

func TestArtifactServiceFolderTreeBuildsHierarchy(t *testing.T) {
	t.Parallel()

	rootID := "folder-root"
	childID := "folder-child"
	repo := &fakeArtifactRepository{
		folders: []FolderNode{
			{ID: childID, ParentID: &rootID, Title: "Child"},
			{ID: rootID, ParentID: nil, Title: "Root"},
		},
	}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	tree, err := svc.FolderTree(testPrincipalContext())
	if err != nil {
		t.Fatalf("FolderTree error: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("tree len = %d, want 1", len(tree))
	}
	if tree[0].ID != rootID || len(tree[0].Children) != 1 || tree[0].Children[0].ID != childID {
		t.Fatalf("tree = %#v, want root -> child hierarchy", tree)
	}
}

func TestArtifactServiceBrowserTreeBuildsMixedHierarchy(t *testing.T) {
	t.Parallel()

	rootID := "folder-root"
	artifactID := "artifact-1"
	artifactType := "page"
	updatedAt := "2026-04-27T00:00:00Z"
	repo := &fakeArtifactRepository{
		browserNodes: []BrowserTreeFlatNode{
			{ID: rootID, Kind: "folder", Title: "Root", Position: 1},
			{ID: artifactID, ParentID: &rootID, Kind: "artifact", Title: "Memo", ArtifactID: &artifactID, ArtifactType: &artifactType, UpdatedAt: &updatedAt, Position: 1},
		},
	}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	tree, err := svc.BrowserTree(testPrincipalContext())
	if err != nil {
		t.Fatalf("BrowserTree error: %v", err)
	}
	if len(tree) != 1 || len(tree[0].Children) != 1 {
		t.Fatalf("tree = %#v, want mixed hierarchy", tree)
	}
	if tree[0].Children[0].ArtifactID == nil || *tree[0].Children[0].ArtifactID != artifactID {
		t.Fatalf("artifact child = %#v, want artifact node", tree[0].Children[0])
	}
}

type fakeArtifactRepository struct {
	artifactByID     map[string]ArtifactResponse
	createdArtifacts []ArtifactResponse
	pageContentByID  map[string]string
	folders          []FolderNode
	browserNodes     []BrowserTreeFlatNode
	searchResults    []ArtifactResponse
	searchParams     *PageSearchParams
}

func (f *fakeArtifactRepository) ListArtifacts(context.Context, ArtifactListParams) ([]ArtifactResponse, error) {
	return nil, nil
}

func (f *fakeArtifactRepository) SearchPageArtifacts(_ context.Context, params PageSearchParams) ([]ArtifactResponse, error) {
	copyParams := params
	f.searchParams = &copyParams
	return f.searchResults, nil
}

func (f *fakeArtifactRepository) GetArtifact(_ context.Context, id string) (ArtifactResponse, error) {
	if f.artifactByID == nil {
		return ArtifactResponse{}, ErrNotFound
	}
	rec, ok := f.artifactByID[id]
	if !ok {
		return ArtifactResponse{}, ErrNotFound
	}
	if rec.Type == "page" {
		if markdown, ok := f.pageContentByID[id]; ok {
			rec.Content = markdown
		}
	}
	return rec, nil
}

func (f *fakeArtifactRepository) CreateArtifact(_ context.Context, rec ArtifactResponse) error {
	f.createdArtifacts = append(f.createdArtifacts, rec)
	return nil
}

func (f *fakeArtifactRepository) UpdateArtifact(context.Context, string, ArtifactPatch) error {
	return nil
}

func (f *fakeArtifactRepository) CreatePageDocument(_ context.Context, artifactID string, markdown string) error {
	if f.pageContentByID == nil {
		f.pageContentByID = map[string]string{}
	}
	f.pageContentByID[artifactID] = markdown
	if rec, ok := f.artifactByID[artifactID]; ok {
		rec.Content = markdown
		f.artifactByID[artifactID] = rec
	}
	return nil
}

func (f *fakeArtifactRepository) SavePageDocument(_ context.Context, artifactID string, markdown string) error {
	if f.pageContentByID == nil {
		f.pageContentByID = map[string]string{}
	}
	f.pageContentByID[artifactID] = markdown
	if rec, ok := f.artifactByID[artifactID]; ok {
		rec.Content = markdown
		f.artifactByID[artifactID] = rec
	}
	return nil
}

func (f *fakeArtifactRepository) SavePageDocumentRevision(_ context.Context, artifactID string, markdown string, revision int64) error {
	if f.artifactByID == nil {
		return ErrNotFound
	}
	rec, ok := f.artifactByID[artifactID]
	if !ok {
		return ErrNotFound
	}
	if rec.Revision >= revision {
		return ErrConflict
	}
	if f.pageContentByID == nil {
		f.pageContentByID = map[string]string{}
	}
	f.pageContentByID[artifactID] = markdown
	rec.Content = markdown
	rec.Revision = revision
	f.artifactByID[artifactID] = rec
	return nil
}

func (f *fakeArtifactRepository) DeleteArtifact(context.Context, string) error { return nil }

func (f *fakeArtifactRepository) ListFolders(context.Context, *string) ([]FolderNode, error) {
	return nil, nil
}
func (f *fakeArtifactRepository) ListAllFolders(context.Context) ([]FolderNode, error) {
	return f.folders, nil
}
func (f *fakeArtifactRepository) ListAllBrowserNodes(context.Context) ([]BrowserTreeFlatNode, error) {
	return f.browserNodes, nil
}
func (f *fakeArtifactRepository) NextNodePosition(context.Context, *string) (int64, error) {
	return 1, nil
}
func (f *fakeArtifactRepository) CreateTreeNode(context.Context, TreeNodeRecord) error { return nil }
func (f *fakeArtifactRepository) UpdateArtifactNodeParent(context.Context, string, *string) error {
	return nil
}
func (f *fakeArtifactRepository) UpdateFolderTitle(context.Context, string, string) error {
	return nil
}
func (f *fakeArtifactRepository) GetFolder(context.Context, string) (FolderNode, error) {
	return FolderNode{}, ErrNotFound
}
func (f *fakeArtifactRepository) FolderBreadcrumbs(context.Context, string) ([]BreadcrumbItem, error) {
	return nil, nil
}

type fakeArtifactFiles struct{}

func (f *fakeArtifactFiles) SaveResource(kind string, filename string, body io.Reader) (StoredArtifactResource, error) {
	_, _ = io.ReadAll(body)
	return StoredArtifactResource{
		StorageKey:       kind + "/resource-1",
		ResourceKind:     kind,
		MIMEType:         "text/plain",
		OriginalFilename: filename,
		SizeBytes:        5,
	}, nil
}

func (f *fakeArtifactFiles) ResourcePath(string) (string, error) { return "", ErrNotFound }

func testPrincipalContext() context.Context {
	return WithPrincipal(context.Background(), Principal{
		UserID:    "00000000-0000-0000-0000-000000000001",
		ActorType: ActorTypeUserSession,
		ActorID:   "00000000-0000-0000-0000-000000000001",
		Email:     "test@aladin.local",
	})
}

func testIntegrationPrincipalContext(scopes ...string) context.Context {
	return WithPrincipal(context.Background(), Principal{
		UserID:    "00000000-0000-0000-0000-000000000001",
		ActorType: ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Email:     "test@aladin.local",
		Scopes:    scopes,
	})
}
