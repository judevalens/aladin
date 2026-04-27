package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

func (s *Server) handleSourcesList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Sources().List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSourcesCreate(w http.ResponseWriter, r *http.Request) {
	var input coreservice.SourceCreateInput
	if err := readJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	rec, err := s.deps.Sources().Create(r.Context(), input)
	if err != nil {
		var requestErr coreservice.BadRequest
		if errors.As(err, &requestErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleSourcesDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid source id"})
		return
	}
	if err := s.deps.Sources().Delete(r.Context(), id); err != nil {
		if errors.Is(err, coreservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Source not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
