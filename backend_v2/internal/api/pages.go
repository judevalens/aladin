package api

import (
	"errors"
	"net/http"

	pageservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerPageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/pages/{id}", s.handlePagesGet)
	mux.HandleFunc("PATCH /api/pages/{id}", s.handlePagesSave)
}

func (s *Server) handlePagesGet(w http.ResponseWriter, r *http.Request) {
	page, err := s.deps.Pages().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, pageservice.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Page not found", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handlePagesSave(w http.ResponseWriter, r *http.Request) {
	var payload pageservice.PageSaveInput
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	page, err := s.deps.Pages().Save(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, pageservice.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Page not found", err)
			return
		}
		if errors.Is(err, pageservice.ErrConflict) {
			writeAPIError(w, r, http.StatusConflict, categoryBadRequest, "Page revision conflict", err)
			return
		}
		var requestErr pageservice.BadRequest
		if errors.As(err, &requestErr) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
