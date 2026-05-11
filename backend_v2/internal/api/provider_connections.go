package api

import (
	"errors"
	"io"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerProviderConnectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/provider-connections/providers", s.handleProviderConnectionProviders)
	mux.HandleFunc("POST /api/provider-connections/{provider}/connect", s.handleProviderConnectionStartConnect)
	mux.HandleFunc("POST /api/provider-connections/sync", s.handleProviderConnectionSync)
	mux.HandleFunc("GET /api/provider-connections", s.handleProviderConnectionList)
	mux.HandleFunc("POST /api/provider-connections/{id}/disconnect", s.handleProviderConnectionDisconnect)
	mux.HandleFunc("POST /api/provider-connections/nango/webhook", s.handleProviderConnectionNangoWebhook)
}

func (s *Server) handleProviderConnectionProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.deps.ProviderConnections().ListProviders(r.Context())
	if err != nil {
		writeProviderConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (s *Server) handleProviderConnectionStartConnect(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	session, err := s.deps.ProviderConnections().StartConnect(r.Context(), coreservice.StartProviderConnectInput{
		Provider: provider,
	})
	if err != nil {
		writeProviderConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleProviderConnectionSync(w http.ResponseWriter, r *http.Request) {
	connections, err := s.deps.ProviderConnections().SyncConnections(r.Context(), coreservice.SyncProviderConnectionsInput{})
	if err != nil {
		writeProviderConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": connections})
}

func (s *Server) handleProviderConnectionList(w http.ResponseWriter, r *http.Request) {
	connections, err := s.deps.ProviderConnections().ListConnections(r.Context())
	if err != nil {
		writeProviderConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": connections})
}

func (s *Server) handleProviderConnectionDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.ProviderConnections().Disconnect(r.Context(), r.PathValue("id")); err != nil {
		writeProviderConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleProviderConnectionNangoWebhook(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeProviderConnectionError(w, r, coreservice.BadRequest("invalid webhook body"))
		return
	}
	if err := s.deps.ProviderConnections().HandleNangoWebhook(r.Context(), coreservice.NangoWebhookInput{
		RawBody:   rawBody,
		Signature: r.Header.Get("X-Nango-Hmac-Sha256"),
	}); err != nil {
		writeProviderConnectionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeProviderConnectionError(w http.ResponseWriter, r *http.Request, err error) {
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
