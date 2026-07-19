package api

import (
	"net/http"
	"strconv"
)

func (s *Server) registerInstrumentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/instruments/search", s.handleInstrumentSearch)
}

// GET /api/instruments/search?q=…&limit=… — ticker typeahead for the command box.
func (s *Server) handleInstrumentSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	hits, err := s.deps.Instruments().Search(r.Context(), q, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}
