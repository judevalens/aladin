package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/app"
	artifactservice "aladin/backend_v2/internal/service"
)

// The content token is the credential a shard iframe carries IN ITS URL, where
// the shard's own JS can read it. These tests pin the boundary that makes that
// safe — it opens shard documents and nothing else — AND that it can still open
// them. (fakeAuthService resolves "content-valid" to a content-token principal
// and "desktop-valid" to a normal session; see server_test.go.)

// shardArtifactService answers Get for one "app" artifact, and — critically —
// enforces the SAME scope gate the real DefaultArtifactService does. Without
// that check a fake would happily serve any principal, and the exact regression
// these tests exist for (a principal that lacks artifacts:read) would be
// invisible here.
type shardArtifactService struct {
	artifactservice.ArtifactService
}

func (shardArtifactService) Get(ctx context.Context, id string) (artifactservice.ArtifactResponse, error) {
	if err := artifactservice.RequireScope(ctx, artifactservice.ScopeArtifactsRead); err != nil {
		return artifactservice.ArtifactResponse{}, err
	}
	if id != "artifact-shard" {
		return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
	}
	return artifactservice.ArtifactResponse{ID: id, Type: "app", Title: "Market Watch"}, nil
}

// emptyShardStore has no built files, so the handler serves NotBuiltHTML — a
// 200 HTML document, which is all these tests need to prove auth + ownership
// resolved. It also gates on scope, mirroring the real store's principal use.
type emptyShardStore struct {
	artifactservice.DocSurfaceStore
}

func (emptyShardStore) ReadFile(ctx context.Context, _, _ string) ([]byte, error) {
	if err := artifactservice.RequireScope(ctx, artifactservice.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return nil, artifactservice.ErrNotFound
}

func contentTokenServer() *Server {
	return NewWithDependencies(":0", app.StaticDependencies{
		AuthSvc:            &fakeAuthService{},
		ArtifactsSvc:       shardArtifactService{},
		DocSurfaceStoreSvc: emptyShardStore{},
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

	// REGRESSION: this originally asserted only "not 401", so a 403 sailed
	// through — the token carries content:read while the content route resolves
	// the artifact through a service requiring artifacts:read, and every shard
	// rendered {"error":"Forbidden"} inside its iframe. Assert it SERVES.
	t.Run("serves a real shard document", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/content/artifact-shard/?access_token=content-valid", nil)
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("content token on its own route = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
			t.Fatalf("expected a shard document, got: %.200s", rec.Body.String())
		}
	})

	t.Run("a session bearer serves it too (no regression for desktop auth)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/content/artifact-shard/?access_token=desktop-valid", nil)
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("session bearer on /content = %d, want 200: %s", rec.Code, rec.Body.String())
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

	// The elevation is scoped to the content route: it must not leak into the
	// principal any other handler sees.
	t.Run("elevation does not escape the content handler", func(t *testing.T) {
		base := artifactservice.WithPrincipal(context.Background(), artifactservice.Principal{
			UserID:    "user-1",
			ActorType: artifactservice.ActorTypeContentToken,
			Scopes:    []string{artifactservice.ScopeContentRead},
		})
		elevated := contentReadContext(base)
		if !artifactservice.HasScope(elevated, artifactservice.ScopeArtifactsRead) {
			t.Fatalf("content route should grant artifacts:read for the request")
		}
		if artifactservice.HasScope(base, artifactservice.ScopeArtifactsRead) {
			t.Fatalf("the original context must be untouched")
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

func TestContentTokenInitialLoadAuthFailure(t *testing.T) {
	for _, query := range []string{"", "?access_token=expired-credential"} {
		t.Run(query, func(t *testing.T) {
			server := contentTokenServer()
			req := httptest.NewRequest(http.MethodGet, "/content/artifact-shard/"+query, nil)
			req.Header.Set("Accept", "text/html")
			rec := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "missing, expired, or no longer valid") || !strings.Contains(body, "Close and reopen") {
				t.Fatalf("missing initial-load recovery guidance: %s", body)
			}
			if strings.Contains(body, "expired-credential") {
				t.Fatal("error page echoed a credential")
			}
			if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatal("auth error must not be cached or send referrers")
			}
		})
	}
}

func TestCookieSessionMintsBoundContentToken(t *testing.T) {
	server := contentTokenServer()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/content-token", nil)
	req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "valid"})
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie session lost identity when minting: %d %s", rec.Code, rec.Body.String())
	}
}

func TestContentTokenCannotAuthenticateAsSessionCookie(t *testing.T) {
	server := contentTokenServer()
	for _, path := range []string{"/api/auth/me", "/content/artifact-shard/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: artifactservice.SessionCookieName, Value: "content-valid"})
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("scoped cookie elevated to a session on %s: %d", path, rec.Code)
		}
	}
}
