package api

import (
	"errors"
	"net/http"
)

type noteRecord struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Label      string         `json:"label"`
	Content    string         `json:"content"`
	Enrichment any            `json:"enrichment"`
	Metadata   map[string]any `json:"metadata"`
	Status     string         `json:"status"`
	CreatedAt  string         `json:"createdAt"`
}

func (s *Server) handleNotesCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label   string `json:"label"`
		Content string `json:"content"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	rec, err := s.deps.Notes.Create(r.Context(), body.Label, body.Content)
	if err != nil {
		if _, ok := err.(badRequest); ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleNotesUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Label   *string `json:"label"`
		Content *string `json:"content"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	rec, err := s.deps.Notes.Update(r.Context(), id, body.Label, body.Content)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Note not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleNotesDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.deps.Notes.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNotesRelated(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := min(intQuery(r, "limit", 8), 30)
	out, err := s.deps.Notes.Related(r.Context(), id, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func derefString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
