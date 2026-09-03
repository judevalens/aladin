// Package httptransport owns the Graph Pane HTTP adapter.
package httptransport

import (
	"net/http"

	"aladin/backend_v2/internal/graphpane"
	"aladin/backend_v2/internal/httpapi"
)

// Register mounts the existing graph-pane route contract.
func Register(mux *http.ServeMux, service graphpane.GraphPaneService) {
	mux.HandleFunc("GET /api/graph-pane", func(w http.ResponseWriter, r *http.Request) {
		artifactID := r.URL.Query().Get("artifact")
		if artifactID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "missing 'artifact' query param", nil)
			return
		}
		out, err := service.ForArtifact(r.Context(), artifactID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	})
}
