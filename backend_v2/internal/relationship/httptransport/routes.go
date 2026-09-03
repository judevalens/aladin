// Package httptransport owns the Relationships HTTP adapter.
package httptransport

import (
	"errors"
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/relationship"
	coreservice "aladin/backend_v2/internal/service"
)

type handler struct {
	service relationship.RelationshipService
}

// Register mounts the existing relationship route contract.
func Register(mux *http.ServeMux, service relationship.RelationshipService) {
	h := handler{service: service}
	mux.HandleFunc("POST /api/relationships", h.create)
	mux.HandleFunc("GET /api/relationships", h.list)
	mux.HandleFunc("DELETE /api/relationships/{id}", h.delete)
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	var item relationship.Relationship
	if err := httpapi.ReadJSON(r, &item); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid json body", err)
		return
	}
	out, err := h.service.Create(r.Context(), item)
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, out)
}

func (h handler) list(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.ListForNode(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if out == nil {
		out = []relationship.Relationship{}
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var badRequest coreservice.BadRequest
	if errors.As(err, &badRequest) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
}
