// Package httptransport owns the Provider Connections HTTP adapter.
package httptransport

import (
	"errors"
	"io"
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/providerconnection"
	coreservice "aladin/backend_v2/internal/service"
)

type handler struct {
	service providerconnection.ProviderConnectionService
}

// Register mounts the existing provider-connections HTTP contract.
func Register(mux *http.ServeMux, service providerconnection.ProviderConnectionService) {
	h := handler{service: service}
	mux.HandleFunc("GET /api/provider-connections/providers", h.listProviders)
	mux.HandleFunc("POST /api/provider-connections/{provider}/connect", h.startConnect)
	mux.HandleFunc("POST /api/provider-connections/sync", h.syncConnections)
	mux.HandleFunc("GET /api/provider-connections", h.listConnections)
	mux.HandleFunc("POST /api/provider-connections/{id}/disconnect", h.disconnect)
	mux.HandleFunc("POST /api/provider-connections/nango/webhook", h.nangoWebhook)
}

func (h handler) listProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListProviders(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (h handler) startConnect(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.StartConnect(r.Context(), providerconnection.StartProviderConnectInput{Provider: r.PathValue("provider")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, session)
}

func (h handler) syncConnections(w http.ResponseWriter, r *http.Request) {
	connections, err := h.service.SyncConnections(r.Context(), providerconnection.SyncProviderConnectionsInput{})
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"connections": connections})
}

func (h handler) listConnections(w http.ResponseWriter, r *http.Request) {
	connections, err := h.service.ListConnections(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"connections": connections})
}

func (h handler) disconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Disconnect(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h handler) nangoWebhook(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, r, coreservice.BadRequest("invalid webhook body"))
		return
	}
	if err := h.service.HandleNangoWebhook(r.Context(), providerconnection.NangoWebhookInput{
		RawBody: rawBody, Signature: r.Header.Get("X-Nango-Hmac-Sha256"),
	}); err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var requestErr coreservice.BadRequest
	switch {
	case errors.Is(err, coreservice.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
	case errors.Is(err, coreservice.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "bad_request", "Forbidden", err)
	case errors.Is(err, coreservice.ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Not found", err)
	case errors.As(err, &requestErr):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
	}
}
