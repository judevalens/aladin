package api

import (
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerGraphPaneRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/graph-pane", s.handleGraphPane)
}

// handleGraphPane serves the "On the graph" side pane, rooted either in the artifact
// you're viewing or in a specific thesis claim:
//
//	GET /api/graph-pane?artifact={artifactId}
//	GET /api/graph-pane?thesis={claimId}
func (s *Server) handleGraphPane(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	artifact := q.Get("artifact")
	thesis := q.Get("thesis")

	var (
		out *coreservice.GraphPane
		err error
	)
	switch {
	case artifact != "":
		out, err = s.deps.GraphPane().ForArtifact(r.Context(), artifact)
	case thesis != "":
		out, err = s.deps.GraphPane().ForThesis(r.Context(), thesis)
	default:
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "missing 'artifact' or 'thesis' query param", nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
