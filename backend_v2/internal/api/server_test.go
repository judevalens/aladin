package api

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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
	server := NewWithDependencies(":0", testDependencies{ArtifactsSvc: service})
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
	server := NewWithDependencies(":0", testDependencies{
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
	server := NewWithDependencies(":0", testDependencies{
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
	server := NewWithDependencies(":0", testDependencies{ArtifactsSvc: service})
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

	server := NewWithDependencies(":0", testDependencies{
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
	server := NewWithDependencies(":0", testDependencies{ArtifactsSvc: service})

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
	server := NewWithDependencies(":0", testDependencies{ArtifactsSvc: service})
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

	server := NewWithDependencies(":0", testDependencies{ArtifactsSvc: &fakeArtifactService{}})
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

	server := NewWithDependencies(":0", testDependencies{
		PagesSvc: &fakePageService{
			page: artifactservice.PageDocument{
				ID:        "artifact-1",
				Title:     "Memo",
				Blocks:    json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"Hello"}],"children":[]}]`),
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
	if !strings.Contains(rec.Body.String(), `"blocks":[`) {
		t.Fatalf("body = %s, want blocks array", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"revision\":7") {
		t.Fatalf("body = %s, want page revision", rec.Body.String())
	}
}

func TestPagesSave(t *testing.T) {
	t.Parallel()

	updatedBlocks := json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"updated"}],"children":[]}]`)
	service := &fakePageService{
		page: artifactservice.PageDocument{
			ID:        "artifact-1",
			Title:     "Memo",
			Blocks:    updatedBlocks,
			Revision:  2,
			UpdatedAt: "2026-05-01T00:00:00Z",
		},
	}
	server := NewWithDependencies(":0", testDependencies{PagesSvc: service})
	req := httptest.NewRequest(http.MethodPatch, "/api/pages/artifact-1", strings.NewReader(`{"blocks":[{"id":"a","type":"paragraph","content":[{"type":"text","text":"updated"}],"children":[]}],"revision":2}`))
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.saved == nil || len(service.saved.Blocks) == 0 {
		t.Fatalf("saved payload = %#v, want updated blocks", service.saved)
	}
	if service.saved == nil || service.saved.Revision != 2 {
		t.Fatalf("saved payload = %#v, want revision 2", service.saved)
	}
}

func TestFilesUpload(t *testing.T) {
	t.Parallel()

	service := &fakeFileService{}
	server := NewWithDependencies(":0", testDependencies{FilesSvc: service})

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

	server := NewWithDependencies(":0", testDependencies{
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

func TestAuthMiddlewareRequiresSession(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestAuthMiddlewareInjectsCurrentUser(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user@example.com") {
		t.Fatalf("body = %s, want current user", rec.Body.String())
	}
}

func TestAuthMiddlewareInjectsCurrentUserFromBearerSession(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer desktop-valid")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user@example.com") {
		t.Fatalf("body = %s, want current user", rec.Body.String())
	}
}

func TestAuthMiddlewareInjectsCurrentUserFromRealtimeAccessToken(t *testing.T) {
	t.Parallel()

	server := &Server{deps: testDependencies{AuthSvc: &fakeAuthService{}}}
	handler := server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := artifactservice.PrincipalFromContext(r.Context()); !ok {
			t.Fatal("principal missing from request context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/events/ws?access_token=desktop-valid", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestDesktopAuthLoginReturnsBearerSession(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc: &fakeAuthService{
			loginSession: artifactservice.AuthSession{
				User:      artifactservice.CurrentUser{ID: "desktop-user", Email: "desktop@example.com"},
				Token:     "desktop-token",
				ExpiresAt: mustTime(t, "2026-06-01T00:00:00Z"),
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/desktop/login", strings.NewReader(`{"email":"desktop@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("set-cookie = %q, want no cookie for desktop auth", rec.Header().Get("Set-Cookie"))
	}
	if !strings.Contains(rec.Body.String(), `"token":"desktop-token"`) {
		t.Fatalf("body = %s, want desktop token", rec.Body.String())
	}
}

func TestDesktopAuthRegisterReturnsBearerSession(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc: &fakeAuthService{
			registerSession: artifactservice.AuthSession{
				User:      artifactservice.CurrentUser{ID: "desktop-user", Email: "desktop@example.com"},
				Token:     "desktop-register-token",
				ExpiresAt: mustTime(t, "2026-06-01T00:00:00Z"),
			},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/desktop/register", strings.NewReader(`{"email":"desktop@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"token":"desktop-register-token"`) {
		t.Fatalf("body = %s, want desktop token", rec.Body.String())
	}
}

func TestAuthLogoutRevokesBearerSession(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{}
	server := NewWithDependencies(":0", testDependencies{AuthSvc: auth})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer desktop-valid")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if auth.logoutToken != "desktop-valid" {
		t.Fatalf("logout token = %q, want desktop-valid", auth.logoutToken)
	}
}

func TestIntegrationTokensCreate(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/integration-tokens",
		strings.NewReader(`{"name":"Claude","scopes":["artifacts:read","artifacts:write"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "aladin_it_test") {
		t.Fatalf("body = %s, want one-time raw token", rec.Body.String())
	}
}

func TestIntegrationTokensRequireUserSession(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/integration-tokens", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestArtifactForbiddenMapsToHTTP403(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:      &fakeAuthService{},
		ArtifactsSvc: &fakeArtifactService{err: artifactservice.ErrForbidden},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/browser/tree", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestPageForbiddenMapsToHTTP403(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:  &fakeAuthService{},
		PagesSvc: &fakePageService{err: artifactservice.ErrForbidden},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/pages/artifact-1", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestFileForbiddenMapsToHTTP403(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:  &fakeAuthService{},
		FilesSvc: &fakeFileService{err: artifactservice.ErrForbidden},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/files/file-1/resource", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestProviderConnectionsRequireAuth(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:                &fakeAuthService{},
		ProviderConnectionsSvc: &fakeProviderConnectionService{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/provider-connections/providers", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestProviderConnectionsProviders(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{
		AuthSvc: &fakeAuthService{},
		ProviderConnectionsSvc: &fakeProviderConnectionService{
			providers: []artifactservice.ProviderDescriptor{
				{
					Provider:          "google",
					Label:             "Google",
					Backend:           "nango",
					ProviderConfigKey: "google-dev",
					Available:         true,
				},
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/provider-connections/providers", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "google") || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("body = %s, want google provider without secrets", rec.Body.String())
	}
}

func TestProviderConnectionsStartConnect(t *testing.T) {
	t.Parallel()

	service := &fakeProviderConnectionService{
		session: artifactservice.ProviderConnectSession{ConnectSessionToken: "nango-session"},
	}
	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:                &fakeAuthService{},
		ProviderConnectionsSvc: service,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/provider-connections/google/connect", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.startProvider != "google" {
		t.Fatalf("start provider = %q, want google", service.startProvider)
	}
}

func TestProviderConnectionsSync(t *testing.T) {
	t.Parallel()

	service := &fakeProviderConnectionService{
		connections: []artifactservice.ProviderConnection{{ID: "conn-1", Provider: "google"}},
	}
	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:                &fakeAuthService{},
		ProviderConnectionsSvc: service,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/provider-connections/sync", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.syncCalls != 1 {
		t.Fatalf("sync calls = %d, want 1", service.syncCalls)
	}
}

func TestProviderConnectionsNangoWebhookIsPublic(t *testing.T) {
	t.Parallel()

	service := &fakeProviderConnectionService{}
	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:                &fakeAuthService{},
		ProviderConnectionsSvc: service,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/provider-connections/nango/webhook", strings.NewReader(`{"type":"auth"}`))
	req.Header.Set("X-Nango-Hmac-Sha256", "signature")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.webhookSignature != "signature" || !strings.Contains(string(service.webhookBody), `"type":"auth"`) {
		t.Fatalf("webhook input = %q %q, want raw body and signature", service.webhookSignature, string(service.webhookBody))
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
	err                error
}

type fakePageService struct {
	page  artifactservice.PageDocument
	saved *artifactservice.PageSaveInput
	err   error
}

type fakeFileService struct {
	uploadInput *artifactservice.FileUploadInput
	resource    artifactservice.FileResource
	err         error
}

func TestAuthResolveReturnsPrincipalForBearerToken(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/resolve", nil)
	req.Header.Set("Authorization", "Bearer desktop-valid")
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got resolvedPrincipalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.UserID != "user-1" || got.ActorType != artifactservice.ActorTypeUserSession {
		t.Fatalf("principal = %#v, want user-1 / user_session", got)
	}
	if got.Scopes == nil {
		t.Fatalf("scopes should be non-nil (empty array, not null)")
	}
}

func TestAuthResolveUnauthenticatedReturns401(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/resolve", nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

// A shard link that navigates the frame off its token-carrying URL used to hand
// the person a raw {"error":"Unauthenticated"} with no hint of the cause. A
// browser navigation now gets an explanation; API callers still get the JSON.
func TestUnauthenticatedBrowserNavigationGetsHTML(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})

	navReq := httptest.NewRequest(http.MethodGet, "/returns", nil)
	navReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	navRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(navRec, navReq)

	if navRec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", navRec.Code)
	}
	if ct := navRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	if body := navRec.Body.String(); !strings.Contains(body, "#/section") {
		t.Errorf("explanation missing the hash-route guidance: %s", body)
	}

	// XHR/fetch and unspecified callers keep the machine-readable shape.
	for _, accept := range []string{"application/json", "*/*", ""} {
		apiReq := httptest.NewRequest(http.MethodGet, "/api/auth/resolve", nil)
		if accept != "" {
			apiReq.Header.Set("Accept", accept)
		}
		apiRec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(apiRec, apiReq)
		if apiRec.Code != http.StatusUnauthorized {
			t.Fatalf("Accept=%q status = %d, want 401", accept, apiRec.Code)
		}
		if got := strings.TrimSpace(apiRec.Body.String()); got != `{"error":"Unauthenticated"}` {
			t.Errorf("Accept=%q body = %s, want the JSON error", accept, got)
		}
	}
}

type fakeAuthService struct {
	loginSession    artifactservice.AuthSession
	registerSession artifactservice.AuthSession
	logoutToken     string
}

type fakeProviderConnectionService struct {
	providers        []artifactservice.ProviderDescriptor
	connections      []artifactservice.ProviderConnection
	session          artifactservice.ProviderConnectSession
	startProvider    string
	syncCalls        int
	webhookBody      []byte
	webhookSignature string
}

func (f *fakeAuthService) Register(context.Context, artifactservice.AuthCredentials, string) (artifactservice.AuthSession, error) {
	if f.registerSession.User.ID != "" {
		return f.registerSession, nil
	}
	return artifactservice.AuthSession{
		User: artifactservice.CurrentUser{ID: "user-registered", Email: "registered@example.com"},
	}, nil
}

func (f *fakeAuthService) Login(context.Context, artifactservice.AuthCredentials, string) (artifactservice.AuthSession, error) {
	if f.loginSession.User.ID != "" {
		return f.loginSession, nil
	}
	return artifactservice.AuthSession{
		User: artifactservice.CurrentUser{ID: "user-1", Email: "user@example.com"},
	}, nil
}

func (f *fakeAuthService) Logout(_ context.Context, token string) error {
	f.logoutToken = token
	return nil
}

func (f *fakeAuthService) CurrentUser(_ context.Context, token string) (artifactservice.CurrentUser, error) {
	if token != "valid" {
		return artifactservice.CurrentUser{}, artifactservice.ErrUnauthenticated
	}
	return artifactservice.CurrentUser{ID: "user-1", Email: "user@example.com"}, nil
}

func (f *fakeAuthService) CreateIntegrationToken(context.Context, artifactservice.IntegrationTokenInput) (artifactservice.CreatedIntegrationToken, error) {
	return artifactservice.CreatedIntegrationToken{
		Token: "aladin_it_test",
		IntegrationToken: artifactservice.IntegrationToken{
			ID:     "token-1",
			Name:   "Claude",
			Scopes: []string{artifactservice.ScopeArtifactsRead, artifactservice.ScopeArtifactsWrite},
			Status: "active",
		},
	}, nil
}

func (f *fakeAuthService) ListIntegrationTokens(context.Context) ([]artifactservice.IntegrationToken, error) {
	return []artifactservice.IntegrationToken{
		{
			ID:     "token-1",
			Name:   "Claude",
			Scopes: []string{artifactservice.ScopeArtifactsRead},
			Status: "active",
		},
	}, nil
}

func (f *fakeAuthService) RevokeIntegrationToken(context.Context, string) error {
	return nil
}

func (f *fakeAuthService) ResolveBearerToken(_ context.Context, token string) (artifactservice.Principal, error) {
	if token == "desktop-valid" || token == "valid" {
		return artifactservice.Principal{
			UserID:           "user-1",
			ActorType:        artifactservice.ActorTypeUserSession,
			ActorID:          "user-1",
			Email:            "user@example.com",
			SessionTokenHash: "session-hash",
		}, nil
	}
	if token == "content-valid" {
		// The scoped shard-document credential: same user, content:read only.
		return artifactservice.Principal{
			UserID:    "user-1",
			ActorType: artifactservice.ActorTypeContentToken,
			ActorID:   "user-1",
			Email:     "user@example.com",
			Scopes:    []string{artifactservice.ScopeContentRead},
		}, nil
	}
	return artifactservice.Principal{}, artifactservice.ErrUnauthenticated
}

func (f *fakeAuthService) MintContentToken(ctx context.Context) (artifactservice.ContentToken, error) {
	principal, err := artifactservice.RequireUserSession(ctx)
	if err != nil || principal.SessionTokenHash == "" {
		return artifactservice.ContentToken{}, artifactservice.ErrUnauthenticated
	}
	return artifactservice.ContentToken{Token: "content-valid", ExpiresAt: "2026-01-01T00:00:00Z"}, nil
}

func (f *fakeProviderConnectionService) ListProviders(context.Context) ([]artifactservice.ProviderDescriptor, error) {
	return f.providers, nil
}

func (f *fakeProviderConnectionService) StartConnect(_ context.Context, input artifactservice.StartProviderConnectInput) (artifactservice.ProviderConnectSession, error) {
	f.startProvider = input.Provider
	return f.session, nil
}

func (f *fakeProviderConnectionService) SyncConnections(context.Context, artifactservice.SyncProviderConnectionsInput) ([]artifactservice.ProviderConnection, error) {
	f.syncCalls++
	return f.connections, nil
}

func (f *fakeProviderConnectionService) ListConnections(context.Context) ([]artifactservice.ProviderConnection, error) {
	return f.connections, nil
}

func (f *fakeProviderConnectionService) Disconnect(context.Context, string) error {
	return nil
}

func (f *fakeProviderConnectionService) GetConnectionCredentials(context.Context, artifactservice.ProviderCredentialRequest) (artifactservice.ProviderCredentials, error) {
	return artifactservice.ProviderCredentials{}, nil
}

func (f *fakeProviderConnectionService) HandleNangoWebhook(_ context.Context, input artifactservice.NangoWebhookInput) error {
	f.webhookBody = input.RawBody
	f.webhookSignature = input.Signature
	return nil
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", value, err)
	}
	return parsed
}

func (f *fakePageService) Get(context.Context, string) (artifactservice.PageDocument, error) {
	if f.err != nil {
		return artifactservice.PageDocument{}, f.err
	}
	return f.page, nil
}

func (f *fakePageService) Save(_ context.Context, _ string, input artifactservice.PageSaveInput) (artifactservice.PageDocument, error) {
	if f.err != nil {
		return artifactservice.PageDocument{}, f.err
	}
	copyInput := input
	f.saved = &copyInput
	return f.page, nil
}

func (f *fakePageService) Attribution(_ context.Context, _ string) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage("{}"), nil
}

func (f *fakePageService) History(_ context.Context, _ string) ([]artifactservice.PageEditEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakePageService) Diff(_ context.Context, _ string) (artifactservice.PageDiff, error) {
	if f.err != nil {
		return artifactservice.PageDiff{}, f.err
	}
	return artifactservice.PageDiff{}, nil
}

func (f *fakeFileService) Upload(_ context.Context, input artifactservice.FileUploadInput, body io.Reader) (artifactservice.FileRecord, error) {
	if f.err != nil {
		return artifactservice.FileRecord{}, f.err
	}
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
	if f.err != nil {
		return artifactservice.FileResource{}, f.err
	}
	return f.resource, nil
}

func (f *fakeArtifactService) List(_ context.Context, params artifactservice.ArtifactListParams) ([]artifactservice.ArtifactResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	copyParams := params
	f.listParams = &copyParams
	return f.list, nil
}

func (f *fakeArtifactService) QueryByProperty(context.Context, artifactservice.PropertyQuery) ([]artifactservice.ArtifactResponse, error) {
	return nil, nil
}

func (f *fakeArtifactService) PropertyFacets(context.Context) ([]artifactservice.PropertyFacet, error) {
	return nil, nil
}

func (f *fakeArtifactService) SearchPages(context.Context, artifactservice.PageSearchParams) ([]artifactservice.ArtifactResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeArtifactService) BrowserTree(context.Context) ([]artifactservice.BrowserTreeNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.browserTree, nil
}

func (f *fakeArtifactService) CreateBrowserNode(_ context.Context, input artifactservice.BrowserNodeCreateInput) (artifactservice.BrowserNodeCreateResponse, error) {
	if f.err != nil {
		return artifactservice.BrowserNodeCreateResponse{}, f.err
	}
	nodeID := "node-created"
	if input.Kind == "artifact" {
		artifactID := "artifact-created"
		return artifactservice.BrowserNodeCreateResponse{
			Node: artifactservice.BrowserNodeResponse{
				ID:         nodeID,
				ParentID:   input.ParentID,
				Kind:       "artifact",
				Title:      input.Title,
				ArtifactID: &artifactID,
				Position:   1,
			},
			Artifact: &artifactservice.ArtifactResponse{
				ID:       artifactID,
				Type:     input.Artifact.Type,
				FolderID: input.ParentID,
				Title:    input.Title,
				Content:  input.Artifact.Content,
				Metadata: map[string]any{},
			},
		}, nil
	}
	return artifactservice.BrowserNodeCreateResponse{
		Node: artifactservice.BrowserNodeResponse{
			ID:       nodeID,
			ParentID: input.ParentID,
			Kind:     "folder",
			Title:    input.Title,
			Position: 1,
		},
	}, nil
}

func (f *fakeArtifactService) DeleteBrowserNode(context.Context, string) (artifactservice.NodeDeleteResult, error) {
	return artifactservice.NodeDeleteResult{}, nil
}

func (f *fakeArtifactService) Get(context.Context, string) (artifactservice.ArtifactResponse, error) {
	if f.err != nil {
		return artifactservice.ArtifactResponse{}, f.err
	}
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Create(_ context.Context, payload artifactservice.ArtifactPayload) (artifactservice.ArtifactCreateResponse, error) {
	if f.err != nil {
		return artifactservice.ArtifactCreateResponse{}, f.err
	}
	f.created = append(f.created, payload)
	artifactID := "artifact-created"
	return artifactservice.ArtifactCreateResponse{
		Artifact: artifactservice.ArtifactResponse{ID: artifactID, Type: payload.Type, FolderID: payload.FolderID, Title: payload.Title, Content: payload.Content, Metadata: map[string]any{}},
		Node: artifactservice.BrowserNodeResponse{
			ID:         artifactID,
			ParentID:   payload.FolderID,
			Kind:       "artifact",
			Title:      payload.Title,
			ArtifactID: &artifactID,
			Position:   1,
		},
	}, nil
}

func (f *fakeArtifactService) Update(context.Context, string, artifactservice.ArtifactPatch) (artifactservice.ArtifactResponse, error) {
	if f.err != nil {
		return artifactservice.ArtifactResponse{}, f.err
	}
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) MoveArtifact(context.Context, string, *string) (artifactservice.ArtifactResponse, error) {
	if f.err != nil {
		return artifactservice.ArtifactResponse{}, f.err
	}
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Delete(context.Context, string) (artifactservice.NodeDeleteResult, error) {
	if f.err != nil {
		return artifactservice.NodeDeleteResult{}, f.err
	}
	return artifactservice.NodeDeleteResult{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Upload(_ context.Context, input artifactservice.ArtifactUploadInput, body io.Reader) (artifactservice.ArtifactResponse, error) {
	if f.err != nil {
		return artifactservice.ArtifactResponse{}, f.err
	}
	_, _ = io.ReadAll(body)
	copyInput := input
	f.uploadInput = &copyInput
	return artifactservice.ArtifactResponse{ID: "artifact-uploaded", Type: input.Type, FolderID: input.FolderID, Title: "Memo", Metadata: map[string]any{}}, nil
}

func (f *fakeArtifactService) Resource(context.Context, string) (artifactservice.ArtifactResource, error) {
	if f.err != nil {
		return artifactservice.ArtifactResource{}, f.err
	}
	return f.resource, nil
}

func (f *fakeArtifactService) ListFolders(context.Context, *string) ([]artifactservice.FolderNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeArtifactService) FolderTree(context.Context) ([]artifactservice.FolderTreeNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.folderTree, nil
}

func (f *fakeArtifactService) CreateFolder(_ context.Context, title string, parentID *string) (artifactservice.FolderNode, error) {
	if f.err != nil {
		return artifactservice.FolderNode{}, f.err
	}
	f.createdFolderTitle = title
	return artifactservice.FolderNode{ID: "folder-1", ParentID: parentID, Title: title}, nil
}

func (f *fakeArtifactService) UpdateFolder(context.Context, string, artifactservice.FolderPatch) (artifactservice.FolderNode, error) {
	if f.err != nil {
		return artifactservice.FolderNode{}, f.err
	}
	return artifactservice.FolderNode{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) GetFolder(context.Context, string) (artifactservice.FolderNode, error) {
	if f.err != nil {
		return artifactservice.FolderNode{}, f.err
	}
	return artifactservice.FolderNode{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) FolderBreadcrumbs(context.Context, string) ([]artifactservice.BreadcrumbItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func stringPtr(value string) *string {
	return &value
}
