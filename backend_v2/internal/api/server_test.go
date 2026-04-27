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
		strings.NewReader(`{"type":"note","title":"Rivian thesis","content":"Supply chain notes"}`),
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

type fakeArtifactService struct {
	list               []artifactservice.ArtifactResponse
	listParams         *artifactservice.ArtifactListParams
	created            []artifactservice.ArtifactPayload
	uploadInput        *artifactservice.ArtifactUploadInput
	resource           artifactservice.ArtifactResource
	createdFolderTitle string
}

func (f *fakeArtifactService) List(_ context.Context, params artifactservice.ArtifactListParams) ([]artifactservice.ArtifactResponse, error) {
	copyParams := params
	f.listParams = &copyParams
	return f.list, nil
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

func (f *fakeArtifactService) CreateFolder(_ context.Context, title string, parentID *string) (artifactservice.FolderNode, error) {
	f.createdFolderTitle = title
	return artifactservice.FolderNode{ID: "folder-1", ParentID: parentID, Title: title}, nil
}

func (f *fakeArtifactService) GetFolder(context.Context, string) (artifactservice.FolderNode, error) {
	return artifactservice.FolderNode{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) FolderBreadcrumbs(context.Context, string) ([]artifactservice.BreadcrumbItem, error) {
	return nil, nil
}
