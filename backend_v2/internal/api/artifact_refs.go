package api

import (
	"errors"
	"net/http"
	"strconv"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerArtifactRefRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/refs/search", s.handleRefSearch)
	mux.HandleFunc("GET /api/artifacts/{id}/refs", s.handleArtifactRefsList)
	mux.HandleFunc("PUT /api/artifacts/{id}/refs", s.handleArtifactRefsSync)
}

// GET /api/refs/search?q=…&limit=… — typeahead for the `#` picker: pages +
// shards, sectioned by kind on the client. limit is per-kind.
func (s *Server) handleRefSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	perKind := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		perKind, _ = strconv.Atoi(raw)
	}
	hits, err := s.deps.ArtifactRefs().Search(r.Context(), principalUserID(r), q, perKind)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}

// GET /api/artifacts/{id}/refs — a page's outgoing `#` refs, resolved to current labels.
func (s *Server) handleArtifactRefsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.ArtifactRefs().ListForArtifact(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, coreservice.ErrInvalidArtifactRef) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "artifact id is required", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/artifacts/{id}/refs {refs:[…]} — reconcile projected `#` refs for a page (the set
// replaces all existing reference rows).
func (s *Server) handleArtifactRefsSync(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Refs []coreservice.ArtifactRef `json:"refs"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.ArtifactRefs().SyncRefs(r.Context(), r.PathValue("id"), payload.Refs); err != nil {
		if errors.Is(err, coreservice.ErrInvalidArtifactRef) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "artifact id is required", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
