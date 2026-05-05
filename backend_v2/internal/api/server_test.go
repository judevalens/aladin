package api

import (
	"aladin/backend_v2/internal/app"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	server := New(":0", &pgxpool.Pool{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestShutdownWithoutRun(t *testing.T) {
	t.Parallel()

	server := New(":0", &pgxpool.Pool{})
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}

func TestArtifactsCreate(t *testing.T) {
	t.Parallel()

	service := &fakeArtifactService{}
	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: service})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/artifacts/",
		strings.NewReader(`{"type":"page","title":"Rivian thesis","content":"Supply chain notes"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(service.created) != 1 {
		t.Fatalf("created calls = %d, want 1", len(service.created))
	}
}

func TestBrowserTreeRoute(t *testing.T) {
	t.Parallel()

	artifactID := "artifact-1"
	artifactType := "page"
	updatedAt := "2026-04-27T00:00:00Z"
	server := NewWithDependencies(":0", app.StaticDependencies{
		ArtifactsSvc: &fakeArtifactService{
			browserTree: []artifactservice.BrowserTreeNode{
				{
					ID:       "folder-root",
					Kind:     "folder",
					Title:    "Root",
					Children: []artifactservice.BrowserTreeNode{{ID: artifactID, ParentID: stringPtr("folder-root"), Kind: "artifact", Title: "Memo", ArtifactID: &artifactID, ArtifactType: &artifactType, UpdatedAt: &updatedAt, Children: []artifactservice.BrowserTreeNode{}}},
				},
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/browser/tree", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"kind\":\"artifact\"") {
		t.Fatalf("body = %s, want mixed browser tree payload", rec.Body.String())
	}
}

func TestArtifactsListByFolder(t *testing.T) {
	t.Parallel()

	folderID := "folder-1"
	server := NewWithDependencies(":0", app.StaticDependencies{
		ArtifactsSvc: &fakeArtifactService{
			list: []artifactservice.ArtifactResponse{{ID: "artifact-1", FolderID: &folderID, Title: "Memo"}},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/artifacts/?folderId=folder-1", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if serverSvc := server.deps.Artifacts().(*fakeArtifactService); serverSvc.listParams == nil || serverSvc.listParams.FolderID == nil || *serverSvc.listParams.FolderID != "folder-1" {
		t.Fatalf("list params = %#v, want folderId=folder-1", serverSvc.listParams)
	}
}

func TestFoldersCreateRoute(t *testing.T) {
	t.Parallel()

	service := &fakeArtifactService{}
	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: service})
	req := httptest.NewRequest(http.MethodPost, "/api/folders/", strings.NewReader(`{"title":"Rivian","parentId":"folder-parent"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if service.createdFolderTitle != "Rivian" {
		t.Fatalf("created folder title = %q, want Rivian", service.createdFolderTitle)
	}
}

func TestFoldersTreeRoute(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", app.StaticDependencies{
		ArtifactsSvc: &fakeArtifactService{
			folderTree: []artifactservice.FolderTreeNode{
				{
					ID:    "folder-root",
					Title: "Root",
					Children: []artifactservice.FolderTreeNode{
						{ID: "folder-child", ParentID: stringPtr("folder-root"), Title: "Child", Children: []artifactservice.FolderTreeNode{}},
					},
				},
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/folders/tree", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"children\"") {
		t.Fatalf("body = %s, want recursive tree payload", rec.Body.String())
	}
}

func TestArtifactsUploadRoute(t *testing.T) {
	t.Parallel()

	service := &fakeArtifactService{}
	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: service})

	var body strings.Builder
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "memo.txt")
	if err != nil {
		t.Fatalf("CreateFormFile error: %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = writer.WriteField("type", "file")
	_ = writer.WriteField("title", "Memo")
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/artifacts/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if service.uploadInput == nil || service.uploadInput.Type != "file" || service.uploadInput.Filename != "memo.txt" {
		t.Fatalf("upload input = %#v, want file memo.txt", service.uploadInput)
	}
}

func TestArtifactsResourceRoute(t *testing.T) {
	t.Parallel()

	tmp, err := os.CreateTemp(t.TempDir(), "artifact-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tmp.WriteString("hello"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	service := &fakeArtifactService{
		resource: artifactservice.ArtifactResource{
			Path:        tmp.Name(),
			ContentType: "text/plain",
		},
	}
	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: service})
	req := httptest.NewRequest(http.MethodGet, "/api/artifacts/artifact-1/resource", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", contentType)
	}
}

func TestLegacyArtifactRoutesRemoved(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: &fakeArtifactService{}})
	paths := []string{
		"/api/notes/",
		"/api/audio/upload",
		"/api/documents/",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
	}
}

func TestPagesGet(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", app.StaticDependencies{
		PagesSvc: &fakePageService{
			page: artifactservice.PageDocument{
				ID:        "artifact-1",
				Title:     "Memo",
				Content:   "# Hello",
				Revision:  7,
				UpdatedAt: "2026-05-01T00:00:00Z",
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/pages/artifact-1", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"content\":\"# Hello\"") {
		t.Fatalf("body = %s, want page content", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"revision\":7") {
		t.Fatalf("body = %s, want page revision", rec.Body.String())
	}
}

func TestPagesSave(t *testing.T) {
	t.Parallel()

	service := &fakePageService{
		page: artifactservice.PageDocument{
			ID:        "artifact-1",
			Title:     "Memo",
			Content:   "updated markdown",
			Revision:  2,
			UpdatedAt: "2026-05-01T00:00:00Z",
		},
	}
	server := NewWithDependencies(":0", app.StaticDependencies{PagesSvc: service})
	req := httptest.NewRequest(http.MethodPatch, "/api/pages/artifact-1", strings.NewReader(`{"content":"updated markdown","revision":2}`))
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.saved == nil || service.saved.Content != "updated markdown" {
		t.Fatalf("saved payload = %#v, want updated markdown", service.saved)
	}
	if service.saved == nil || service.saved.Revision != 2 {
		t.Fatalf("saved payload = %#v, want revision 2", service.saved)
	}
}

func TestFilesUpload(t *testing.T) {
	t.Parallel()

	service := &fakeFileService{}
	server := NewWithDependencies(":0", app.StaticDependencies{FilesSvc: service})

	var body strings.Builder
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "memo.txt")
	if err != nil {
		t.Fatalf("CreateFormFile error: %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if service.uploadInput == nil || service.uploadInput.Filename != "memo.txt" {
		t.Fatalf("upload input = %#v, want memo.txt", service.uploadInput)
	}
}

func TestFilesResourceRoute(t *testing.T) {
	t.Parallel()

	tmp, err := os.CreateTemp(t.TempDir(), "file-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tmp.WriteString("hello"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	server := NewWithDependencies(":0", app.StaticDependencies{
		FilesSvc: &fakeFileService{
			resource: artifactservice.FileResource{
				Path:        tmp.Name(),
				ContentType: "text/plain",
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/files/file-1/resource", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", contentType)
	}
}

type fakeArtifactService struct {
	list               []artifactservice.ArtifactResponse
	listParams         *artifactservice.ArtifactListParams
	browserTree        []artifactservice.BrowserTreeNode
	folderTree         []artifactservice.FolderTreeNode
	created            []artifactservice.ArtifactPayload
	uploadInput        *artifactservice.ArtifactUploadInput
	resource           artifactservice.ArtifactResource
	createdFolderTitle string
}

type fakePageService struct {
	page  artifactservice.PageDocument
	saved *artifactservice.PageSaveInput
}

type fakeFileService struct {
	uploadInput *artifactservice.FileUploadInput
	resource    artifactservice.FileResource
}

func (f *fakePageService) Get(context.Context, string) (artifactservice.PageDocument, error) {
	return f.page, nil
}

func (f *fakePageService) Save(_ context.Context, _ string, input artifactservice.PageSaveInput) (artifactservice.PageDocument, error) {
	copyInput := input
	f.saved = &copyInput
	return f.page, nil
}

func (f *fakeFileService) Upload(_ context.Context, input artifactservice.FileUploadInput, body io.Reader) (artifactservice.FileRecord, error) {
	_, _ = io.ReadAll(body)
	copyInput := input
	f.uploadInput = &copyInput
	return artifactservice.FileRecord{
		ID:         "file-uploaded",
		URL:        "/api/files/file-uploaded/resource",
		UploadedAt: "2026-05-01T00:00:00Z",
	}, nil
}

func (f *fakeFileService) Resource(context.Context, string) (artifactservice.FileResource, error) {
	return f.resource, nil
}

func (f *fakeArtifactService) List(_ context.Context, params artifactservice.ArtifactListParams) ([]artifactservice.ArtifactResponse, error) {
	copyParams := params
	f.listParams = &copyParams
	return f.list, nil
}

func (f *fakeArtifactService) BrowserTree(context.Context) ([]artifactservice.BrowserTreeNode, error) {
	return f.browserTree, nil
}

func (f *fakeArtifactService) Get(context.Context, string) (artifactservice.ArtifactResponse, error) {
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Create(_ context.Context, payload artifactservice.ArtifactPayload) (artifactservice.ArtifactResponse, error) {
	f.created = append(f.created, payload)
	return artifactservice.ArtifactResponse{ID: "artifact-created", Type: payload.Type, FolderID: payload.FolderID, Title: payload.Title, Content: payload.Content, Metadata: map[string]any{}}, nil
}

func (f *fakeArtifactService) Update(context.Context, string, artifactservice.ArtifactPatch) (artifactservice.ArtifactResponse, error) {
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Delete(context.Context, string) error {
	return artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Upload(_ context.Context, input artifactservice.ArtifactUploadInput, body io.Reader) (artifactservice.ArtifactResponse, error) {
	_, _ = io.ReadAll(body)
	copyInput := input
	f.uploadInput = &copyInput
	return artifactservice.ArtifactResponse{ID: "artifact-uploaded", Type: input.Type, FolderID: input.FolderID, Title: "Memo", Metadata: map[string]any{}}, nil
}

func (f *fakeArtifactService) Resource(context.Context, string) (artifactservice.ArtifactResource, error) {
	return f.resource, nil
}

func (f *fakeArtifactService) ListFolders(context.Context, *string) ([]artifactservice.FolderNode, error) {
	return nil, nil
}

func (f *fakeArtifactService) FolderTree(context.Context) ([]artifactservice.FolderTreeNode, error) {
	return f.folderTree, nil
}

func (f *fakeArtifactService) CreateFolder(_ context.Context, title string, parentID *string) (artifactservice.FolderNode, error) {
	f.createdFolderTitle = title
	return artifactservice.FolderNode{ID: "folder-1", ParentID: parentID, Title: title}, nil
}

func (f *fakeArtifactService) UpdateFolder(context.Context, string, artifactservice.FolderPatch) (artifactservice.FolderNode, error) {
	return artifactservice.FolderNode{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) GetFolder(context.Context, string) (artifactservice.FolderNode, error) {
	return artifactservice.FolderNode{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) FolderBreadcrumbs(context.Context, string) ([]artifactservice.BreadcrumbItem, error) {
	return nil, nil
}

func stringPtr(value string) *string {
	return &value
}
