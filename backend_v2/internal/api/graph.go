package api

import (
	"context"
	"net/http"
	"time"
)

// graphReadTimeout bounds a graph-neighbours read so a slow/down Neo4j can't tie up the handler
// for the driver's 60s default connection-acquisition timeout.
const graphReadTimeout = 5 * time.Second

// registerGraphRoutes mounts the read path for the Neo4j connection lens. The write/projection
// path lives in the worker; this only serves an entity's neighbourhood for the (gated) graph UI.
func (s *Server) registerGraphRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/graph/entity/{id}/neighbors", s.handleGraphEntityNeighbors)
}

// handleGraphEntityNeighbors returns an entity's local graph: the entities it co-occurs
// with. 503 when Neo4j isn't configured (GraphReader is nil).
func (s *Server) handleGraphEntityNeighbors(w http.ResponseWriter, r *http.Request) {
	reader := s.deps.GraphReader()
	if reader == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, categoryServiceError, "graph not configured", nil)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "entity id required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), graphReadTimeout)
	defer cancel()
	nb, err := reader.Neighbors(ctx, id, min(intQuery(r, "limit", 25), 100))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, nb)
}
