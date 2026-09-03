// Package httptransport owns the Insights HTTP adapter.
package httptransport

import (
	"net/http"
	"strconv"
	"strings"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/insights"
)

type handler struct{ service insights.InsightService }

// Register mounts the existing insights route contract.
func Register(mux *http.ServeMux, service insights.InsightService) {
	h := handler{service: service}
	mux.HandleFunc("GET /api/insights/", h.list)
	mux.HandleFunc("GET /api/insights/stats", h.stats)
	mux.HandleFunc("POST /api/insights/{id}/accept", h.updateStatus("accepted"))
	mux.HandleFunc("POST /api/insights/{id}/dismiss", h.updateStatus("dismissed"))
}

func (h handler) list(w http.ResponseWriter, r *http.Request) {
	params := insights.InsightListParams{
		Limit:  min(intQuery(r, "limit", 30), 100),
		Offset: intQuery(r, "offset", 0),
		Type:   r.URL.Query().Get("type"),
		Status: firstNonEmpty(r.URL.Query().Get("status"), "pending"),
	}
	out, err := h.service.List(r.Context(), params)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h handler) stats(w http.ResponseWriter, r *http.Request) {
	out, _ := h.service.Stats(r.Context())
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h handler) updateStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.service.UpdateStatus(r.Context(), r.PathValue("id"), status); err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
