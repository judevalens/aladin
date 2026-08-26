package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerReadingPositionRoutes(mux *http.ServeMux) {
	// The synced "where am I in this document" row. Clients READ via the sync
	// replica; PUT is the (debounced) position report, GET is for the writer's
	// confirmation + debugging.
	mux.HandleFunc("PUT /api/reading-positions/{artifactId}", s.handleReadingPositionPut)
	mux.HandleFunc("GET /api/reading-positions/{artifactId}", s.handleReadingPositionGet)
}

// PUT /api/reading-positions/{artifactId} {page} — report the current page
// (last-write-wins). Returns the committed row with its seq so the caller can
// apply it through the same guard the live frame uses.
func (s *Server) handleReadingPositionPut(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Page int64 `json:"page"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	pos, err := s.deps.ReadingPositions().Put(r.Context(), principalUserID(r), r.PathValue("artifactId"), payload.Page)
	if err != nil {
		s.writeReadingPositionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pos)
}

// GET /api/reading-positions/{artifactId} — the stored position (404 if none).
func (s *Server) handleReadingPositionGet(w http.ResponseWriter, r *http.Request) {
	pos, ok, err := s.deps.ReadingPositions().Get(r.Context(), principalUserID(r), r.PathValue("artifactId"))
	if err != nil {
		s.writeReadingPositionError(w, r, err)
		return
	}
	if !ok {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "No reading position", coreservice.ErrReadingPositionNotFound)
		return
	}
	writeJSON(w, http.StatusOK, pos)
}

func (s *Server) writeReadingPositionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, coreservice.ErrInvalidReadingPositionInput):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	case errors.Is(err, coreservice.ErrReadingPositionNotFound):
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Artifact not found", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}
