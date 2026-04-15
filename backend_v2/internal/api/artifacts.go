package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type artifactRecord struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Label      string         `json:"label"`
	Content    string         `json:"content"`
	SourceURL  *string        `json:"sourceUrl"`
	Enrichment any            `json:"enrichment"`
	Metadata   map[string]any `json:"metadata"`
	ChildCount int            `json:"childCount"`
	CreatedAt  string         `json:"createdAt"`
}

func (s *Server) handleArtifactsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Artifacts.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleArtifactChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := min(intQuery(r, "limit", 100), 500)
	offset := intQuery(r, "offset", 0)

	out, err := s.deps.Artifacts.Children(r.Context(), id, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleArtifactsCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Label     string `json:"label"`
		Content   string `json:"content"`
		SourceURL string `json:"sourceUrl"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	payload.Type = strings.TrimSpace(payload.Type)
	payload.Label = strings.TrimSpace(payload.Label)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.SourceURL = strings.TrimSpace(payload.SourceURL)
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
	if err := s.deps.Artifacts.Create(r.Context(), payload.ID, payload.Type, payload.Label, payload.Content, payload.SourceURL); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handleArtifactsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "artifact id is required"})
		return
	}
	if err := s.deps.Artifacts.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type scanner interface{ Scan(dest ...any) error }

func scanArtifactRow(row scanner, withChildCount bool) (artifactRecord, error) {
	var rec artifactRecord
	var sourceURL *string
	var enrichment []byte
	var metadata []byte
	var createdAt time.Time
	rec.Metadata = map[string]any{}
	if withChildCount {
		if err := row.Scan(&rec.ID, &rec.Type, &rec.Label, &rec.Content, &sourceURL, &enrichment, &metadata, &createdAt, &rec.ChildCount); err != nil {
			return rec, err
		}
	} else {
		if err := row.Scan(&rec.ID, &rec.Type, &rec.Label, &rec.Content, &sourceURL, &enrichment, &metadata, &createdAt); err != nil {
			return rec, err
		}
	}
	if len(enrichment) > 0 {
		_ = json.Unmarshal(enrichment, &rec.Enrichment)
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &rec.Metadata)
	}
	rec.SourceURL = sourceURL
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	return rec, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
