package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", s.handleAuthRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/desktop/register", s.handleDesktopAuthRegister)
	mux.HandleFunc("POST /api/auth/desktop/login", s.handleDesktopAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)
	mux.HandleFunc("GET /api/auth/resolve", s.handleAuthResolve)
	// Mints the scoped credential the app puts in a shard iframe's URL, so the
	// session bearer never rides a document a shard's JS can read.
	mux.HandleFunc("POST /api/auth/content-token", s.handleContentTokenMint)
	mux.HandleFunc("GET /api/integration-tokens", s.handleIntegrationTokensList)
	mux.HandleFunc("POST /api/integration-tokens", s.handleIntegrationTokensCreate)
	mux.HandleFunc("POST /api/integration-tokens/{id}/revoke", s.handleIntegrationTokensRevoke)
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User coreservice.CurrentUser `json:"user"`
}

type desktopAuthResponse struct {
	User      coreservice.CurrentUser `json:"user"`
	Token     string                  `json:"token"`
	ExpiresAt time.Time               `json:"expiresAt"`
}

type integrationTokenRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func (s *Server) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := readJSON(r, &input); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	session, err := s.deps.Auth().Register(r.Context(), coreservice.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusCreated, authResponse{User: session.User})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := readJSON(r, &input); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	session, err := s.deps.Auth().Login(r.Context(), coreservice.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	setSessionCookie(w, r, session)
	writeJSON(w, http.StatusOK, authResponse{User: session.User})
}

func (s *Server) handleDesktopAuthRegister(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := readJSON(r, &input); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	session, err := s.deps.Auth().Register(r.Context(), coreservice.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, desktopAuthResponse{
		User:      session.User,
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
	})
}

func (s *Server) handleDesktopAuthLogin(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := readJSON(r, &input); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	session, err := s.deps.Auth().Login(r.Context(), coreservice.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, desktopAuthResponse{
		User:      session.User,
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(coreservice.SessionCookieName)
	if err == nil {
		_ = s.deps.Auth().Logout(r.Context(), cookie.Value)
	}
	if token := bearerTokenFromAuthorizationHeader(r.Header.Get("Authorization")); token != "" {
		_ = s.deps.Auth().Logout(r.Context(), token)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleContentTokenMint(w http.ResponseWriter, r *http.Request) {
	token, err := s.deps.Auth().MintContentToken(r.Context())
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryInternal, "could not mint content token", err)
		return
	}
	writeJSON(w, http.StatusOK, token)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user, ok := coreservice.CurrentUserFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: user})
}

type resolvedPrincipalResponse struct {
	UserID    string   `json:"userId"`
	ActorType string   `json:"actorType"`
	ActorID   string   `json:"actorId"`
	Email     string   `json:"email"`
	Scopes    []string `json:"scopes"`
}

// handleAuthResolve returns the resolved principal for whatever credential
// the caller presented (session cookie or bearer token). The auth middleware
// already populated the context principal and 401s non-public routes with no
// valid credential, so this handler is a thin read. The Hocuspocus
// onAuthenticate hook (services/blocknote) calls this to turn a connection's
// token into an identity for awareness + write authorization.
func (s *Server) handleAuthResolve(w http.ResponseWriter, r *http.Request) {
	principal, ok := coreservice.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	scopes := principal.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	writeJSON(w, http.StatusOK, resolvedPrincipalResponse{
		UserID:    principal.UserID,
		ActorType: principal.ActorType,
		ActorID:   principal.ActorID,
		Email:     principal.Email,
		Scopes:    scopes,
	})
}

func (s *Server) handleIntegrationTokensList(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.deps.Auth().ListIntegrationTokens(r.Context())
	if err != nil {
		writeAuthManagementError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (s *Server) handleIntegrationTokensCreate(w http.ResponseWriter, r *http.Request) {
	var input integrationTokenRequest
	if err := readJSON(r, &input); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	created, err := s.deps.Auth().CreateIntegrationToken(r.Context(), coreservice.IntegrationTokenInput{
		Name:      input.Name,
		Scopes:    input.Scopes,
		ExpiresAt: input.ExpiresAt,
	})
	if err != nil {
		writeAuthManagementError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleIntegrationTokensRevoke(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Auth().RevokeIntegrationToken(r.Context(), r.PathValue("id")); err != nil {
		writeAuthManagementError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var requestErr coreservice.BadRequest
	switch {
	case errors.Is(err, coreservice.ErrUnauthenticated):
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Invalid email or password", err)
	case errors.As(err, &requestErr):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}

func writeAuthManagementError(w http.ResponseWriter, r *http.Request, err error) {
	var requestErr coreservice.BadRequest
	switch {
	case errors.Is(err, coreservice.ErrUnauthenticated):
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", err)
	case errors.Is(err, coreservice.ErrForbidden):
		writeAPIError(w, r, http.StatusForbidden, categoryBadRequest, "Forbidden", err)
	case errors.Is(err, coreservice.ErrNotFound):
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", err)
	case errors.As(err, &requestErr):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session coreservice.AuthSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     coreservice.SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     coreservice.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func bearerTokenFromAuthorizationHeader(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
