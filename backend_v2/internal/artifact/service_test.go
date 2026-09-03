package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
)

func TestArtifactServiceCreatePageIgnoresBlocks(t *testing.T) {
	t.Parallel()

	summary := "  useful memo  "
	repo := &fakeArtifactRepository{}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	// M8c: page content is owned by the collab Y.Doc; Create ignores blocks
	// and always materializes an empty document (content arrives via the
	// editor or the MCP bridge).
	blocks := json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"Rivian supply chain memo"}],"children":[]}]`)
	result, err := svc.Create(testPrincipalContext(), ArtifactPayload{
		Type:    "page",
		Title:   "Rivian supply chain memo",
		Blocks:  blocks,
		Summary: &summary,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rec := result.Artifact
	if rec.ID == "" || !strings.HasPrefix(rec.ID, "artifact-") {
		t.Fatalf("id = %q, want artifact-*", rec.ID)
	}
	if rec.Content != "" {
		t.Fatalf("page artifact Content should be empty, got %q", rec.Content)
	}
	if string(rec.Blocks) != "[]" {
		t.Fatalf("page should be created with empty blocks, got %s", string(rec.Blocks))
	}
	stored := repo.pagesByID[rec.ID]
	if stored == nil {
		t.Fatalf("expected page document to be created for id %q", rec.ID)
	}
	if stored.searchText != "" {
		t.Fatalf("search_text = %q, want empty (blocks ignored)", stored.searchText)
	}
}

func TestArtifactServiceCreatePageIgnoresMalformedBlocks(t *testing.T) {
	t.Parallel()
	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	// Blocks are ignored entirely now, so even malformed blocks don't error —
	// the page is simply created empty.
	result, err := svc.Create(testPrincipalContext(), ArtifactPayload{
		Type:   "page",
		Title:  "Bad",
		Blocks: json.RawMessage(`{"not":"an array"}`),
	})
	if err != nil {
		t.Fatalf("Create error = %v, want nil (blocks ignored)", err)
	}
	if string(result.Artifact.Blocks) != "[]" {
		t.Fatalf("blocks = %s, want []", string(result.Artifact.Blocks))
	}
}

func TestArtifactServiceCreatePageRequiresTitle(t *testing.T) {
	t.Parallel()
	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	_, err := svc.Create(testPrincipalContext(), ArtifactPayload{
		Type: "page",
	})
	var requestErr apperror.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Create without title error = %v, want apperror.BadRequest", err)
	}
}

func TestArtifactServiceCreateBoardStoresSnapshot(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	result, err := svc.Create(testPrincipalContext(), ArtifactPayload{
		Type:    "board",
		Title:   "Lesson sketch",
		Content: `{"document":{"store":{}}}`,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rec := result.Artifact
	if rec.Type != "board" {
		t.Fatalf("type = %q, want board", rec.Type)
	}
	if rec.Content != `{"document":{"store":{}}}` {
		t.Fatalf("content = %q, want tldraw snapshot", rec.Content)
	}
	if repo.pagesByID[rec.ID] != nil {
		t.Fatalf("board should not create a page document row")
	}
}

func TestArtifactServiceRequiresPrincipal(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	if _, err := svc.Create(context.Background(), ArtifactPayload{Type: "page", Title: "Memo"}); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("Create error = %v, want auth.ErrUnauthenticated", err)
	}
	if _, err := svc.BrowserTree(context.Background()); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("BrowserTree error = %v, want auth.ErrUnauthenticated", err)
	}
}

func TestArtifactServiceReadOnlyTokenCannotWrite(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	ctx := testIntegrationPrincipalContext(auth.ScopeArtifactsRead)

	if _, err := svc.BrowserTree(ctx); err != nil {
		t.Fatalf("BrowserTree read-only error = %v, want nil", err)
	}
	if _, err := svc.Create(ctx, ArtifactPayload{Type: "page", Title: "Memo"}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("Create read-only error = %v, want auth.ErrForbidden", err)
	}
	if _, err := svc.Delete(ctx, "artifact-1"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("Delete read-only error = %v, want auth.ErrForbidden", err)
	}
	if _, err := svc.CreateFolder(ctx, "Folder", nil); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("CreateFolder read-only error = %v, want auth.ErrForbidden", err)
	}
	if _, err := svc.MoveArtifact(ctx, "artifact-1", nil); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("MoveArtifact read-only error = %v, want auth.ErrForbidden", err)
	}
}

func TestArtifactServiceSearchPagesValidatesAndClamps(t *testing.T) {
	t.Parallel()

	repo := &fakeArtifactRepository{
		searchResults: []ArtifactResponse{{ID: "page-1", Type: "page", Title: "Match"}},
	}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	if _, err := svc.SearchPages(testIntegrationPrincipalContext(auth.ScopeArtifactsWrite), PageSearchParams{Query: "memo"}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("SearchPages without read scope error = %v, want auth.ErrForbidden", err)
	}
	if _, err := svc.SearchPages(testPrincipalContext(), PageSearchParams{Query: "   "}); err == nil {
		t.Fatal("SearchPages empty query error = nil, want apperror.BadRequest")
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

func TestArtifactServiceMoveArtifactCanMoveToFolderAndRoot(t *testing.T) {
	t.Parallel()

	initialFolder := "folder-old"
	targetFolder := "folder-new"
	repo := &fakeArtifactRepository{
		artifactByID: map[string]ArtifactResponse{
			"artifact-1": {ID: "artifact-1", Type: "page", FolderID: &initialFolder, Title: "Doc"},
		},
		containers: map[string]FolderNode{
			targetFolder: {ID: targetFolder, Title: "Docs"},
		},
	}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})
	ctx := testPrincipalContext()

	moved, err := svc.MoveArtifact(ctx, "artifact-1", &targetFolder)
	if err != nil {
		t.Fatalf("MoveArtifact to folder error: %v", err)
	}
	if moved.FolderID == nil || *moved.FolderID != targetFolder {
		t.Fatalf("folder = %v, want %q", moved.FolderID, targetFolder)
	}
	if moved.Seq != 1 {
		t.Fatalf("seq = %d, want 1", moved.Seq)
	}

	rooted, err := svc.MoveArtifact(ctx, "artifact-1", nil)
	if err != nil {
		t.Fatalf("MoveArtifact to root error: %v", err)
	}
	if rooted.FolderID != nil {
		t.Fatalf("folder = %v, want root", rooted.FolderID)
	}
}

func TestArtifactServiceCreateLinkRequiresSourceURL(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	_, err := svc.Create(testPrincipalContext(), ArtifactPayload{Type: "link", Title: "Saved"})
	if err == nil {
		t.Fatal("Create error = nil, want apperror.BadRequest")
	}
	var requestErr apperror.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Create error = %T, want apperror.BadRequest", err)
	}
}

func TestArtifactServiceEmptyIDsAreNotFound(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	if _, err := svc.Get(testPrincipalContext(), " "); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("Get error = %v, want apperror.ErrNotFound", err)
	}
	if _, err := svc.Update(testPrincipalContext(), " ", ArtifactPatch{}); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("Update error = %v, want apperror.ErrNotFound", err)
	}
	if _, err := svc.Delete(testPrincipalContext(), " "); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("Delete error = %v, want apperror.ErrNotFound", err)
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

func TestArtifactServiceUploadRemovesBytesWhenAggregateWriteFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("aggregate write failed")
	files := &fakeArtifactFiles{}
	svc := NewArtifactService(&fakeArtifactRepository{createGraphErr: wantErr}, files)

	_, err := svc.Upload(testPrincipalContext(), ArtifactUploadInput{
		Type:     "file",
		Filename: "report.txt",
	}, strings.NewReader("hello"))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Upload error = %v, want %v", err, wantErr)
	}
	if len(files.deleted) != 1 || files.deleted[0] != "file/resource-1" {
		t.Fatalf("deleted = %#v, want stored resource compensation", files.deleted)
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

type fakePageStore struct {
	blocks     json.RawMessage
	searchText string
	revision   int64
}

type fakeArtifactRepository struct {
	artifactByID     map[string]ArtifactResponse
	createdArtifacts []ArtifactResponse
	pagesByID        map[string]*fakePageStore
	folders          []FolderNode
	containers       map[string]FolderNode
	browserNodes     []BrowserTreeFlatNode
	searchResults    []ArtifactResponse
	propertyQuery    *PropertyQuery
	propertyResults  []ArtifactResponse
	propertyFacets   []PropertyFacet
	searchParams     *PageSearchParams
	createGraphErr   error
}

func (f *fakeArtifactRepository) ListArtifacts(context.Context, ArtifactListParams) ([]ArtifactResponse, error) {
	return nil, nil
}

func (f *fakeArtifactRepository) QueryArtifactsByProperty(_ context.Context, params PropertyQuery) ([]ArtifactResponse, error) {
	copyParams := params
	f.propertyQuery = &copyParams
	return f.propertyResults, nil
}

func (f *fakeArtifactRepository) PropertyFacets(_ context.Context) ([]PropertyFacet, error) {
	return f.propertyFacets, nil
}

func (f *fakeArtifactRepository) SearchPageArtifacts(_ context.Context, params PageSearchParams) ([]ArtifactResponse, error) {
	copyParams := params
	f.searchParams = &copyParams
	return f.searchResults, nil
}

func (f *fakeArtifactRepository) GetArtifact(_ context.Context, id string) (ArtifactResponse, error) {
	if f.artifactByID == nil {
		return ArtifactResponse{}, apperror.ErrNotFound
	}
	rec, ok := f.artifactByID[id]
	if !ok {
		return ArtifactResponse{}, apperror.ErrNotFound
	}
	if rec.Type == "page" {
		if page, ok := f.pagesByID[id]; ok && page != nil {
			rec.Blocks = page.blocks
			rec.Revision = page.revision
		} else {
			rec.Blocks = json.RawMessage(`[]`)
		}
		rec.Content = ""
	}
	return rec, nil
}

func (f *fakeArtifactRepository) LightNode(_ context.Context, id string) (BrowserNodeResponse, error) {
	return BrowserNodeResponse{ID: id, Seq: 1}, nil
}

func (f *fakeArtifactRepository) CreateArtifact(_ context.Context, rec ArtifactResponse) error {
	f.createdArtifacts = append(f.createdArtifacts, rec)
	return nil
}

func (f *fakeArtifactRepository) CreateArtifactGraph(_ context.Context, rec ArtifactResponse, node TreeNodeRecord, pageBlocks json.RawMessage, pageSearchText string) error {
	if f.createGraphErr != nil {
		return f.createGraphErr
	}
	f.createdArtifacts = append(f.createdArtifacts, rec)
	if pageBlocks != nil {
		if f.pagesByID == nil {
			f.pagesByID = map[string]*fakePageStore{}
		}
		f.pagesByID[rec.ID] = &fakePageStore{
			blocks:     pageBlocks,
			searchText: pageSearchText,
		}
	}
	artifactType := rec.Type
	f.browserNodes = append(f.browserNodes, BrowserTreeFlatNode{
		ID:           node.ID,
		ParentID:     node.ParentID,
		Kind:         node.Kind,
		Title:        rec.Title,
		ArtifactID:   node.ArtifactID,
		ArtifactType: &artifactType,
		Position:     node.Position,
	})
	return nil
}

func (f *fakeArtifactRepository) UpdateArtifactGraph(context.Context, string, ArtifactPatch) error {
	return nil
}

func (f *fakeArtifactRepository) PageBlockAttribution(_ context.Context, _ string) (json.RawMessage, error) {
	return json.RawMessage("{}"), nil
}

func (f *fakeArtifactRepository) SavePageBlocks(_ context.Context, artifactID string, blocks json.RawMessage, searchText string, expectedRev int64) (int64, error) {
	if f.artifactByID == nil {
		return 0, apperror.ErrNotFound
	}
	if _, ok := f.artifactByID[artifactID]; !ok {
		return 0, apperror.ErrNotFound
	}
	if f.pagesByID == nil {
		f.pagesByID = map[string]*fakePageStore{}
	}
	page, ok := f.pagesByID[artifactID]
	if !ok {
		page = &fakePageStore{}
		f.pagesByID[artifactID] = page
	}
	if expectedRev > 0 && page.revision >= expectedRev {
		return 0, apperror.ErrConflict
	}
	page.blocks = blocks
	page.searchText = searchText
	if expectedRev > 0 {
		page.revision = expectedRev
	} else {
		page.revision++
	}
	return page.revision, nil
}

func (f *fakeArtifactRepository) GetPageBlocks(_ context.Context, artifactID string) (json.RawMessage, int64, error) {
	if f.pagesByID == nil {
		return nil, 0, apperror.ErrNotFound
	}
	page, ok := f.pagesByID[artifactID]
	if !ok {
		return nil, 0, apperror.ErrNotFound
	}
	return page.blocks, page.revision, nil
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
func (f *fakeArtifactRepository) DeleteBrowserNode(context.Context, string) error      { return nil }
func (f *fakeArtifactRepository) UpdateArtifactNodeParent(_ context.Context, id string, parentID *string) error {
	if f.artifactByID == nil {
		return apperror.ErrNotFound
	}
	rec, ok := f.artifactByID[id]
	if !ok {
		return apperror.ErrNotFound
	}
	rec.FolderID = parentID
	f.artifactByID[id] = rec
	return nil
}
func (f *fakeArtifactRepository) UpdateFolderTitle(context.Context, string, string) error {
	return nil
}
func (f *fakeArtifactRepository) GetFolder(context.Context, string) (FolderNode, error) {
	return FolderNode{}, apperror.ErrNotFound
}

func (f *fakeArtifactRepository) GetContainer(_ context.Context, id string) (FolderNode, error) {
	if f.containers != nil {
		if rec, ok := f.containers[id]; ok {
			return rec, nil
		}
	}
	return FolderNode{}, apperror.ErrNotFound
}
func (f *fakeArtifactRepository) FolderBreadcrumbs(context.Context, string) ([]BreadcrumbItem, error) {
	return nil, nil
}

type fakeArtifactFiles struct{ deleted []string }

func (f *fakeArtifactFiles) SaveResource(kind string, filename string, _ string, body io.Reader) (StoredArtifactResource, error) {
	_, _ = io.ReadAll(body)
	return StoredArtifactResource{
		StorageKey:       kind + "/resource-1",
		ResourceKind:     kind,
		MIMEType:         "text/plain",
		OriginalFilename: filename,
		SizeBytes:        5,
	}, nil
}

func (f *fakeArtifactFiles) ResourcePath(string) (string, error) { return "", apperror.ErrNotFound }
func (f *fakeArtifactFiles) DeleteResource(key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func testPrincipalContext() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:    "00000000-0000-0000-0000-000000000001",
		ActorType: auth.ActorTypeUserSession,
		ActorID:   "00000000-0000-0000-0000-000000000001",
		Email:     "test@aladin.local",
	})
}

func testIntegrationPrincipalContext(scopes ...string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:    "00000000-0000-0000-0000-000000000001",
		ActorType: auth.ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Email:     "test@aladin.local",
		Scopes:    scopes,
	})
}
