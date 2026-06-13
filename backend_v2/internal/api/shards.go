package api

import (
	"net/http"
	"strings"

	"aladin/backend_v2/internal/docsurface"
)

func (s *Server) registerShardRoutes(mux *http.ServeMux) {
	// Build status for the live work-pane view. Authed (authMiddleware) + ownership
	// scoped via Artifacts().Get; the realtime build-status events seed/update the
	// same shape live, this GET seeds it when a shard tab first opens.
	mux.HandleFunc("GET /api/shards/{id}/build-state", s.handleShardBuildState)
	// The parsed anchor manifest (for the host: provenance, deep links, and the
	// M3 bridge's ref grant). 404 if the shard has no anchors.json.
	mux.HandleFunc("GET /api/shards/{id}/manifest", s.handleShardManifest)
}

// ownedShard verifies the path id names an "app" artifact owned by the principal,
// writing the appropriate error and returning false otherwise.
func (s *Server) ownedShard(w http.ResponseWriter, r *http.Request, pageID string) bool {
	if pageID == "" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return false
	}
	rec, err := s.deps.Artifacts().Get(r.Context(), pageID)
	if err != nil {
		if writeAccessError(w, r, err) {
			return false
		}
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", err)
		return false
	}
	if rec.Type != "app" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return false
	}
	return true
}

func (s *Server) handleShardBuildState(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	if !s.ownedShard(w, r, pageID) {
		return
	}
	channel := docsurface.ParseChannel(r.URL.Query().Get("channel"))
	st, err := s.deps.ShardBuild().State(r.Context(), pageID, channel)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryInternal, "build state unavailable", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleShardManifest(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	if !s.ownedShard(w, r, pageID) {
		return
	}
	data, err := s.deps.DocSurfaceStore().ReadFile(r.Context(), pageID, docsurface.ManifestFileName)
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "no manifest", nil)
		return
	}
	m, err := docsurface.ParseManifest(data)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, categoryServiceError, "manifest is not valid JSON", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
