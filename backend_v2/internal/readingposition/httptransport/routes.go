// Package httptransport owns the public Reading Position HTTP adapter.
package httptransport

import (
	"context"
	"errors"
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/readingposition"
)

type Service interface {
	Put(context.Context, string, string, int64) (readingposition.ReadingPosition, error)
	Get(context.Context, string, string) (readingposition.ReadingPosition, bool, error)
}

type routes struct{ service Service }

func Register(mux *http.ServeMux, service Service) {
	routes := routes{service: service}
	// The synced "where am I in this document" row. Clients READ via the sync
	// replica; PUT is the (debounced) position report, GET is for the writer's
	// confirmation + debugging.
	mux.HandleFunc("PUT /api/reading-positions/{artifactId}", routes.handlePut)
	mux.HandleFunc("GET /api/reading-positions/{artifactId}", routes.handleGet)
}

// PUT /api/reading-positions/{artifactId} {page} — report the current page
// (last-write-wins). Returns the committed row with its seq so the caller can
// apply it through the same guard the live frame uses.
func (routes routes) handlePut(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Page int64 `json:"page"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	pos, err := routes.service.Put(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("artifactId"), payload.Page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, pos)
}

// GET /api/reading-positions/{artifactId} — the stored position (404 if none).
func (routes routes) handleGet(w http.ResponseWriter, r *http.Request) {
	pos, ok, err := routes.service.Get(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("artifactId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !ok {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "No reading position", readingposition.ErrReadingPositionNotFound)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, pos)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, readingposition.ErrInvalidReadingPositionInput):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
	case errors.Is(err, readingposition.ErrReadingPositionNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Artifact not found", err)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
	}
}
