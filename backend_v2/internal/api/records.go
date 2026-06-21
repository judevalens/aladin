package api

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleRecordsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Records().List(r.Context())
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRecordChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := min(intQuery(r, "limit", 100), 500)
	offset := intQuery(r, "offset", 0)

	out, err := s.deps.Records().Children(r.Context(), id, limit, offset)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRecordsCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Label     string `json:"label"`
		Content   string `json:"content"`
		SourceURL string `json:"sourceUrl"`
		ParentID  string `json:"parentId"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	payload.Type = strings.TrimSpace(payload.Type)
	payload.Label = strings.TrimSpace(payload.Label)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.SourceURL = strings.TrimSpace(payload.SourceURL)
	payload.ParentID = strings.TrimSpace(payload.ParentID)
	if payload.Type == "" {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "type is required", nil)
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
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "content is required", nil)
		return
	}
	if err := s.deps.Records().Create(r.Context(), payload.ID, payload.Type, payload.Label, payload.Content, payload.SourceURL, payload.ParentID); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleRecordsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "record id is required", nil)
		return
	}
	if err := s.deps.Records().Delete(r.Context(), id); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/records/{id}/similar?limit=N — records vector-near the given one.
func (s *Server) handleRecordSimilar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "record id is required", nil)
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	out, err := s.deps.Records().Similar(r.Context(), id, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/records/{id}/retry — re-drive a failed record back into the pipeline.
func (s *Server) handleRecordsRetry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "record id is required", nil)
		return
	}
	retried, err := s.deps.Records().Retry(r.Context(), id)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"retried": retried})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
