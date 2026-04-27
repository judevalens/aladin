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
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSourcesCreate(w http.ResponseWriter, r *http.Request) {
	var input coreservice.SourceCreateInput
	if err := readJSON(r, &input); err != nil {
		writeDecodeError(w, r, err)
		return
	}

	rec, err := s.deps.Sources().Create(r.Context(), input)
	if err != nil {
		var requestErr coreservice.BadRequest
		if errors.As(err, &requestErr) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleSourcesDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "Invalid source id", err)
		return
	}
	if err := s.deps.Sources().Delete(r.Context(), id); err != nil {
		if errors.Is(err, coreservice.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Source not found", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
