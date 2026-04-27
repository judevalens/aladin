package api

import (
	"net/http"
	"strings"
)

func (s *Server) handleRecordsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Records().List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	payload.Type = strings.TrimSpace(payload.Type)
	payload.Label = strings.TrimSpace(payload.Label)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.SourceURL = strings.TrimSpace(payload.SourceURL)
	payload.ParentID = strings.TrimSpace(payload.ParentID)
	if payload.Type == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type is required"})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	if err := s.deps.Records().Create(r.Context(), payload.ID, payload.Type, payload.Label, payload.Content, payload.SourceURL, payload.ParentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleRecordsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "record id is required"})
		return
	}
	if err := s.deps.Records().Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
