package api

import (
	"errors"
	"net/http"
	"time"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", s.handleAuthRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User coreservice.CurrentUser `json:"user"`
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

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(coreservice.SessionCookieName)
	if err == nil {
		_ = s.deps.Auth().Logout(r.Context(), cookie.Value)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user, ok := coreservice.CurrentUserFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: user})
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

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
