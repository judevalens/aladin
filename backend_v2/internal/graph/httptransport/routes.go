package httptransport

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/httpapi"
)

// graphReadTimeout bounds a graph-neighbours read so a slow/down Neo4j can't tie up the handler
// for the driver's 60s default connection-acquisition timeout.
const graphReadTimeout = 5 * time.Second

// registerGraphRoutes mounts the read path for the Neo4j connection lens. The write/projection
// path lives in the worker; this only serves an entity's neighbourhood for the (gated) graph UI.
type routes struct{ reader graph.GraphReader }

func Register(mux *http.ServeMux, reader graph.GraphReader) {
	r := routes{reader: reader}
	mux.HandleFunc("GET /api/graph/entity/{id}/neighbors", r.handleGraphEntityNeighbors)
}

// handleGraphEntityNeighbors returns an entity's local graph: the entities it co-occurs
// with. 503 when Neo4j isn't configured (GraphReader is nil).
func (h routes) handleGraphEntityNeighbors(w http.ResponseWriter, r *http.Request) {
	if h.reader == nil {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "service_error", "graph not configured", nil)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "entity id required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), graphReadTimeout)
	defer cancel()
	nb, err := h.reader.Neighbors(ctx, id, min(intQuery(r, "limit", 25), 100))
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, nb)
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
