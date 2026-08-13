package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"aladin/backend_v2/internal/service"
)

func TestBearerAuthRejectsMissingMalformedAndUnknownTokens(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{tokens: map[string]service.Principal{
		"valid": testPrincipal(service.ScopeArtifactsRead),
	}}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := bearerAuth(auth, next)

	cases := []struct {
		name          string
		authorization string
	}{
		{name: "missing"},
		{name: "malformed", authorization: "Token valid"},
		{name: "unknown", authorization: "Bearer nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestBearerAuthInjectsPrincipal(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{tokens: map[string]service.Principal{
		"valid": testPrincipal(service.ScopeArtifactsRead),
	}}
	var observed service.Principal
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		observed, ok = service.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("principal missing from request context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := bearerAuth(auth, next)
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer valid")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if observed.UserID != "user-1" || observed.ActorType != service.ActorTypeIntegrationToken {
		t.Fatalf("principal = %#v, want integration principal for user-1", observed)
	}
}

type fakeAuthService struct {
	tokens map[string]service.Principal
}

func (f *fakeAuthService) Register(context.Context, service.AuthCredentials, string) (service.AuthSession, error) {
	return service.AuthSession{}, nil
}

func (f *fakeAuthService) Login(context.Context, service.AuthCredentials, string) (service.AuthSession, error) {
	return service.AuthSession{}, nil
}

func (f *fakeAuthService) Logout(context.Context, string) error {
	return nil
}

func (f *fakeAuthService) CurrentUser(context.Context, string) (service.CurrentUser, error) {
	return service.CurrentUser{}, service.ErrUnauthenticated
}

func (f *fakeAuthService) CreateIntegrationToken(context.Context, service.IntegrationTokenInput) (service.CreatedIntegrationToken, error) {
	return service.CreatedIntegrationToken{}, service.ErrForbidden
}

func (f *fakeAuthService) ListIntegrationTokens(context.Context) ([]service.IntegrationToken, error) {
	return nil, service.ErrForbidden
}

func (f *fakeAuthService) RevokeIntegrationToken(context.Context, string) error {
	return service.ErrForbidden
}

func (f *fakeAuthService) ResolveBearerToken(_ context.Context, token string) (service.Principal, error) {
	principal, ok := f.tokens[token]
	if !ok {
		return service.Principal{}, service.ErrUnauthenticated
	}
	return principal, nil
}

func (f *fakeAuthService) MintContentToken(context.Context) (service.ContentToken, error) {
	return service.ContentToken{}, service.ErrUnauthenticated
}
