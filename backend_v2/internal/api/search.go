package api

import (
	"net/http"
	"strconv"
)

func (s *Server) registerSearchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/search", s.handleGlobalSearch)
}

// GET /api/search?q=…&limit=… — the global command-box search: federated, sectioned.
func (s *Server) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	out, err := s.deps.Search().Search(r.Context(), principalUserID(r), q, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
