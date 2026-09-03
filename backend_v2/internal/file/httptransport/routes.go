package httptransport

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"aladin/backend_v2/internal/file"
	"aladin/backend_v2/internal/httpapi"
	coreservice "aladin/backend_v2/internal/service"
)

type routes struct{ service file.FileService }

func Register(mux *http.ServeMux, service file.FileService) {
	r := routes{service: service}
	mux.HandleFunc("POST /api/files/upload", r.handleFilesUpload)
	mux.HandleFunc("GET /api/files/{id}/resource", r.handleFilesResource)
}

func (h routes) handleFilesUpload(w http.ResponseWriter, r *http.Request) {
	upload, header, err := r.FormFile("file")
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "no file provided", err)
		return
	}
	defer upload.Close()

	rec, err := h.service.Upload(r.Context(), file.FileUploadInput{
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
	}, upload)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		var requestErr coreservice.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, rec)
}

func (h routes) handleFilesResource(w http.ResponseWriter, r *http.Request) {
	resource, err := h.service.Resource(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, coreservice.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "File resource not found", err)
			return
		}
		var requestErr coreservice.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	if resource.ContentType == "" {
		if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(resource.Path))); contentType != "" {
			resource.ContentType = contentType
		}
	}
	if resource.ContentType != "" {
		w.Header().Set("Content-Type", resource.ContentType)
	}
	http.ServeFile(w, r, resource.Path)
}

func writeAccessError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, coreservice.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
		return true
	case errors.Is(err, coreservice.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "bad_request", "Forbidden", err)
		return true
	default:
		return false
	}
}
