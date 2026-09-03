// Package httptransport owns the Feed HTTP adapter.
package httptransport

import (
	"net/http"
	"strconv"
	"strings"

	"aladin/backend_v2/internal/feed"
	"aladin/backend_v2/internal/httpapi"
)

type handler struct{ service feed.FeedService }

// Register mounts the existing feed route contract.
func Register(mux *http.ServeMux, service feed.FeedService) {
	h := handler{service: service}
	mux.HandleFunc("GET /api/feed/", h.list)
	mux.HandleFunc("GET /api/feed/topics", h.topics)
	mux.HandleFunc("GET /api/feed/sources", h.sources)
	mux.HandleFunc("POST /api/feed/{id}/save", h.updateStatus("saved"))
	mux.HandleFunc("POST /api/feed/{id}/dismiss", h.updateStatus("dismissed"))
	mux.HandleFunc("POST /api/feed/{id}/unsave", h.updateStatus(""))
}

func (h handler) list(w http.ResponseWriter, r *http.Request) {
	params := feed.FeedListParams{
		Limit:      min(intQuery(r, "limit", 50), 200),
		Offset:     intQuery(r, "offset", 0),
		SourceType: r.URL.Query().Get("source_type"),
		Topic:      r.URL.Query().Get("topic"),
		SavedOnly:  r.URL.Query().Get("saved") == "true",
		Sort:       firstNonEmpty(r.URL.Query().Get("sort"), "recent"),
	}
	out, err := h.service.List(r.Context(), params)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h handler) topics(w http.ResponseWriter, r *http.Request) {
	out, _ := h.service.Topics(r.Context())
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h handler) sources(w http.ResponseWriter, r *http.Request) {
	out, _ := h.service.Sources(r.Context())
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
