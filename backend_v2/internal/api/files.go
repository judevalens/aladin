package api

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	artifactservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerFileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/files/upload", s.handleFilesUpload)
	mux.HandleFunc("GET /api/files/{id}/resource", s.handleFilesResource)
}

func (s *Server) handleFilesUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "no file provided", err)
		return
	}
	defer file.Close()

	rec, err := s.deps.Files().Upload(r.Context(), artifactservice.FileUploadInput{
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
	}, file)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleFilesResource(w http.ResponseWriter, r *http.Request) {
	resource, err := s.deps.Files().Resource(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "File resource not found", err)
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
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
