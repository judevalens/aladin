package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/app"
)

// The content token is the credential a shard iframe carries IN ITS URL, where
// the shard's own JS can read it. These tests pin the boundary that makes that
// safe: it opens shard documents and nothing else. (fakeAuthService resolves
// "content-valid" to a content-token principal and "desktop-valid" to a normal
// session — see server_test.go.)

func contentTokenServer() *Server {
	return NewWithDependencies(":0", app.StaticDependencies{
		AuthSvc:      &fakeAuthService{},
		ArtifactsSvc: &fakeArtifactService{},
	})
}

func TestContentToken_ServesContentButNotAPI(t *testing.T) {
	t.Parallel()
	server := contentTokenServer()

	t.Run("rejected on /api even though the user is real", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer content-valid")
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET /api/auth/me with a content token = %d, want 401", rec.Code)
		}
	})

	t.Run("rejected as ?access_token on the realtime websocket route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/events/ws?access_token=content-valid", nil)
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("WS handshake with a content token = %d, want 401", rec.Code)
		}
	})

	t.Run("accepted on /content — reaches the handler, not the auth wall", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/content/artifact-missing/?access_token=content-valid", nil)
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		// The fixture has no such shard, so a non-401 (the handler's own 404) is
		// the success signal: auth let it through.
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("content token was rejected on its own route")
		}
	})

	t.Run("a session bearer still works on /api", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer desktop-valid")
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/auth/me with a session bearer = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestContentTokenMintRoute(t *testing.T) {
	t.Parallel()
	server := contentTokenServer()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/content-token", nil)
	req.Header.Set("Authorization", "Bearer desktop-valid")
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/auth/content-token = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "content-valid") {
		t.Fatalf("mint response missing the token: %s", rec.Body.String())
	}
}
