package httptransport

import (
	"errors"
	"net/http"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/page"
)

type routes struct{ service page.Service }

func Register(mux *http.ServeMux, service page.Service) {
	h := routes{service: service}
	mux.HandleFunc("GET /api/pages/{id}", h.get)
	mux.HandleFunc("GET /api/pages/{id}/attribution", h.attribution)
	mux.HandleFunc("GET /api/pages/{id}/history", h.history)
	mux.HandleFunc("GET /api/pages/{id}/history/{entryId}/diff", h.historyDiff)
	mux.HandleFunc("PATCH /api/pages/{id}", h.save)
}

func (h routes) historyDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := h.service.Diff(r.Context(), r.PathValue("entryId"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "History entry not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, diff)
}

func (h routes) history(w http.ResponseWriter, r *http.Request) {
	entries, err := h.service.History(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Page not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, entries)
}

func (h routes) attribution(w http.ResponseWriter, r *http.Request) {
	attr, err := h.service.Attribution(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Page not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(attr)
}

func (h routes) get(w http.ResponseWriter, r *http.Request) {
	document, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Page not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, document)
}

func (h routes) save(w http.ResponseWriter, r *http.Request) {
	var payload page.PageSaveInput
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	document, err := h.service.Save(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		switch {
		case errors.Is(err, apperror.ErrNotFound):
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Page not found", err)
		case errors.Is(err, apperror.ErrConflict):
			httpapi.WriteError(w, r, http.StatusConflict, "bad_request", "Page revision conflict", err)
		default:
			var requestErr apperror.BadRequest
			if errors.As(err, &requestErr) {
				httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		}
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, document)
}

func writeAccessError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
		return true
	case errors.Is(err, auth.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "bad_request", "Forbidden", err)
		return true
	default:
		return false
	}
}
