package httptransport

import (
	"net/http"
	"strconv"
	"strings"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/record"
)

type routes struct{ service record.RecordService }

func Register(mux *http.ServeMux, service record.RecordService) {
	r := routes{service: service}
	mux.HandleFunc("GET /api/records/", r.handleRecordsList)
	mux.HandleFunc("POST /api/records/", r.handleRecordsCreate)
	mux.HandleFunc("DELETE /api/records/{id}", r.handleRecordsDelete)
	mux.HandleFunc("POST /api/records/{id}/retry", r.handleRecordsRetry)
	mux.HandleFunc("GET /api/records/{id}/similar", r.handleRecordSimilar)
	mux.HandleFunc("GET /api/records/{id}/children", r.handleRecordChildren)
}

func (h routes) handleRecordsList(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handleRecordChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := min(intQuery(r, "limit", 100), 500)
	offset := intQuery(r, "offset", 0)

	out, err := h.service.Children(r.Context(), id, limit, offset)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handleRecordsCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Label     string `json:"label"`
		Content   string `json:"content"`
		SourceURL string `json:"sourceUrl"`
		ParentID  string `json:"parentId"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	payload.Type = strings.TrimSpace(payload.Type)
	payload.Label = strings.TrimSpace(payload.Label)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.SourceURL = strings.TrimSpace(payload.SourceURL)
	payload.ParentID = strings.TrimSpace(payload.ParentID)
	if payload.Type == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "type is required", nil)
		return
	}
	if payload.Label == "" {
		switch {
		case payload.SourceURL != "":
			payload.Label = payload.SourceURL
		case payload.Content != "":
			payload.Label = payload.Content
		default:
			payload.Label = strings.Title(strings.ReplaceAll(payload.Type, "_", " "))
		}
	}
	if payload.Content == "" {
		payload.Content = payload.Label
	}
	if payload.Content == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "content is required", nil)
		return
	}
	if err := h.service.Create(r.Context(), payload.ID, payload.Type, payload.Label, payload.Content, payload.SourceURL, payload.ParentID); err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (h routes) handleRecordsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "record id is required", nil)
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/records/{id}/similar?limit=N — records vector-near the given one.
func (h routes) handleRecordSimilar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "record id is required", nil)
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	out, err := h.service.Similar(r.Context(), id, limit)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// POST /api/records/{id}/retry — re-drive a failed record back into the pipeline.
func (h routes) handleRecordsRetry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "record id is required", nil)
		return
	}
	retried, err := h.service.Retry(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"retried": retried})
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
