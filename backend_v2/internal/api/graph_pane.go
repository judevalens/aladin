package api

import (
	"net/http"
)

func (s *Server) registerGraphPaneRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/graph-pane", s.handleGraphPane)
}

// handleGraphPane serves the "On the graph" side pane, rooted either in the artifact
// you're viewing or in a specific thesis claim:
//
//	GET /api/graph-pane?artifact={artifactId}
func (s *Server) handleGraphPane(w http.ResponseWriter, r *http.Request) {
	artifact := r.URL.Query().Get("artifact")
	if artifact == "" {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "missing 'artifact' query param", nil)
		return
	}

	out, err := s.deps.GraphPane().ForArtifact(r.Context(), artifact)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
