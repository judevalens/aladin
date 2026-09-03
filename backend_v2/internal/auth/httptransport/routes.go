package httptransport

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/httpapi"
)

type routes struct{ service auth.AuthService }

func Register(mux *http.ServeMux, service auth.AuthService) {
	r := routes{service: service}
	mux.HandleFunc("POST /api/auth/register", r.handleAuthRegister)
	mux.HandleFunc("POST /api/auth/login", r.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/desktop/register", r.handleDesktopAuthRegister)
	mux.HandleFunc("POST /api/auth/desktop/login", r.handleDesktopAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", r.handleAuthLogout)
	mux.HandleFunc("GET /api/auth/me", r.handleAuthMe)
	mux.HandleFunc("GET /api/auth/resolve", r.handleAuthResolve)
	// Mints the scoped credential the app puts in a shard iframe's URL, so the
	// session bearer never rides a document a shard's JS can read.
	mux.HandleFunc("POST /api/auth/content-token", r.handleContentTokenMint)
	mux.HandleFunc("GET /api/integration-tokens", r.handleIntegrationTokensList)
	mux.HandleFunc("POST /api/integration-tokens", r.handleIntegrationTokensCreate)
	mux.HandleFunc("POST /api/integration-tokens/{id}/revoke", r.handleIntegrationTokensRevoke)
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User auth.CurrentUser `json:"user"`
}

type desktopAuthResponse struct {
	User      auth.CurrentUser `json:"user"`
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expiresAt"`
}

type integrationTokenRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func (h routes) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	session, err := h.service.Register(r.Context(), auth.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	setSessionCookie(w, r, session)
	httpapi.WriteJSON(w, http.StatusCreated, authResponse{User: session.User})
}

func (h routes) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	session, err := h.service.Login(r.Context(), auth.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	setSessionCookie(w, r, session)
	httpapi.WriteJSON(w, http.StatusOK, authResponse{User: session.User})
}

func (h routes) handleDesktopAuthRegister(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	session, err := h.service.Register(r.Context(), auth.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, desktopAuthResponse{
		User:      session.User,
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
	})
}

func (h routes) handleDesktopAuthLogin(w http.ResponseWriter, r *http.Request) {
	var input authRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	session, err := h.service.Login(r.Context(), auth.AuthCredentials{
		Email:    input.Email,
		Password: input.Password,
	}, r.UserAgent())
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, desktopAuthResponse{
		User:      session.User,
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
	})
}

func (h routes) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}
	if token := bearerTokenFromAuthorizationHeader(r.Header.Get("Authorization")); token != "" {
		_ = h.service.Logout(r.Context(), token)
	}
	clearSessionCookie(w, r)
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h routes) handleContentTokenMint(w http.ResponseWriter, r *http.Request) {
	token, err := h.service.MintContentToken(r.Context())
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not mint content token", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, token)
}

func (h routes) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.CurrentUserFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", auth.ErrUnauthenticated)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, authResponse{User: user})
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
func (h routes) handleAuthResolve(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", auth.ErrUnauthenticated)
		return
	}
	scopes := principal.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	httpapi.WriteJSON(w, http.StatusOK, resolvedPrincipalResponse{
		UserID:    principal.UserID,
		ActorType: principal.ActorType,
		ActorID:   principal.ActorID,
		Email:     principal.Email,
		Scopes:    scopes,
	})
}

func (h routes) handleIntegrationTokensList(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.service.ListIntegrationTokens(r.Context())
	if err != nil {
		writeAuthManagementError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (h routes) handleIntegrationTokensCreate(w http.ResponseWriter, r *http.Request) {
	var input integrationTokenRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	created, err := h.service.CreateIntegrationToken(r.Context(), auth.IntegrationTokenInput{
		Name:      input.Name,
		Scopes:    input.Scopes,
		ExpiresAt: input.ExpiresAt,
	})
	if err != nil {
		writeAuthManagementError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, created)
}

func (h routes) handleIntegrationTokensRevoke(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RevokeIntegrationToken(r.Context(), r.PathValue("id")); err != nil {
		writeAuthManagementError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var requestErr auth.BadRequest
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Invalid email or password", err)
	case errors.As(err, &requestErr):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
	}
}

func writeAuthManagementError(w http.ResponseWriter, r *http.Request, err error) {
	var requestErr auth.BadRequest
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
	case errors.Is(err, auth.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "bad_request", "Forbidden", err)
	case errors.Is(err, auth.ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Not found", err)
	case errors.As(err, &requestErr):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
	}
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session auth.AuthSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
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
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
}

func writeAccessError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
		return true
	case errors.Is(err, auth.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "bad_request", "Forbidden", err)
		return true
	default:
		return false
	}
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
