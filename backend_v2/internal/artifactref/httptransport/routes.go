// Package httptransport owns the public HTTP adapter for Artifact References.
package httptransport

import (
	"errors"
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/artifactref"
	"aladin/backend_v2/internal/httpapi"
)

type routes struct {
	service artifactref.ArtifactRefService
}

// Register installs the existing reference routes without changing their contracts.
func Register(mux *http.ServeMux, service artifactref.ArtifactRefService) {
	routes := routes{service: service}
	mux.HandleFunc("GET /api/refs/search", routes.handleRefSearch)
	mux.HandleFunc("GET /api/artifacts/{id}/refs", routes.handleArtifactRefsList)
	mux.HandleFunc("PUT /api/artifacts/{id}/refs", routes.handleArtifactRefsSync)
}

// GET /api/refs/search?q=…&limit=… — typeahead for the `#` picker: pages +
// shards, sectioned by kind on the client. limit is per-kind.
func (routes routes) handleRefSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	perKind := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		perKind, _ = strconv.Atoi(raw)
	}
	hits, err := routes.service.Search(r.Context(), httpapi.PrincipalUserID(r), q, perKind)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, hits)
}

// GET /api/artifacts/{id}/refs — a page's outgoing `#` refs, resolved to current labels.
func (routes routes) handleArtifactRefsList(w http.ResponseWriter, r *http.Request) {
	out, err := routes.service.ListForArtifact(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, artifactref.ErrInvalidArtifactRef) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "artifact id is required", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// PUT /api/artifacts/{id}/refs {refs:[…]} — reconcile projected `#` refs for a page (the set
// replaces all existing reference rows).
func (routes routes) handleArtifactRefsSync(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Refs []artifactref.ArtifactRef `json:"refs"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	if err := routes.service.SyncRefs(r.Context(), r.PathValue("id"), payload.Refs); err != nil {
		if errors.Is(err, artifactref.ErrInvalidArtifactRef) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "artifact id is required", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
