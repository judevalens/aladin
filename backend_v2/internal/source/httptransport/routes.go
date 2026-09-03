// Package httptransport owns the Sources HTTP adapter.
package httptransport

import (
	"errors"
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/source"

	"github.com/google/uuid"
)

type handler struct{ service source.SourceService }

// Register mounts the existing source route contract.
func Register(mux *http.ServeMux, service source.SourceService) {
	h := handler{service: service}
	mux.HandleFunc("GET /api/sources/", h.list)
	mux.HandleFunc("POST /api/sources/", h.create)
	mux.HandleFunc("DELETE /api/sources/{id}", h.delete)
}

func (h handler) list(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	var input source.SourceCreateInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	record, err := h.service.Create(r.Context(), input)
	if err != nil {
		var requestErr coreservice.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, record)
}

func (h handler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "Invalid source id", err)
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		if errors.Is(err, coreservice.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Source not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
