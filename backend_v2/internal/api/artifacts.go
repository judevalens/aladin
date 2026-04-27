package api

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	artifactservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerArtifactRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/artifacts/", s.handleArtifactsList)
	mux.HandleFunc("POST /api/artifacts/", s.handleArtifactsCreate)
	mux.HandleFunc("POST /api/artifacts/upload", s.handleArtifactsUpload)
	mux.HandleFunc("GET /api/artifacts/{id}", s.handleArtifactsGet)
	mux.HandleFunc("PATCH /api/artifacts/{id}", s.handleArtifactsUpdate)
	mux.HandleFunc("DELETE /api/artifacts/{id}", s.handleArtifactsDelete)
	mux.HandleFunc("GET /api/artifacts/{id}/resource", s.handleArtifactsResource)

	mux.HandleFunc("GET /api/folders/", s.handleFoldersList)
	mux.HandleFunc("POST /api/folders/", s.handleFoldersCreate)
	mux.HandleFunc("GET /api/folders/{id}", s.handleFoldersGet)
	mux.HandleFunc("GET /api/folders/{id}/breadcrumbs", s.handleFoldersBreadcrumbs)
}

func (s *Server) handleArtifactsList(w http.ResponseWriter, r *http.Request) {
	params := artifactservice.ArtifactListParams{
		FolderID: stringQueryPtr(r, "folderId"),
	}
	out, err := s.deps.Artifacts().List(r.Context(), params)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleArtifactsGet(w http.ResponseWriter, r *http.Request) {
	rec, err := s.deps.Artifacts().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Artifact not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleArtifactsCreate(w http.ResponseWriter, r *http.Request) {
	var payload artifactservice.ArtifactPayload
	if err := readJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	rec, err := s.deps.Artifacts().Create(r.Context(), payload)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleArtifactsUpdate(w http.ResponseWriter, r *http.Request) {
	var patch artifactservice.ArtifactPatch
	if err := readJSON(r, &patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	rec, err := s.deps.Artifacts().Update(r.Context(), r.PathValue("id"), patch)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Artifact or folder not found"})
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleArtifactsDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Artifacts().Delete(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Artifact not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleArtifactsUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()

	typeValue := strings.TrimSpace(r.FormValue("type"))
	title := stringFormPtr(r, "title")
	summary := stringFormPtr(r, "summary")
	folderID := stringFormPtr(r, "folderId")
	rec, err := s.deps.Artifacts().Upload(r.Context(), artifactservice.ArtifactUploadInput{
		Type:     typeValue,
		Filename: header.Filename,
		Title:    title,
		Summary:  summary,
		FolderID: folderID,
	}, file)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleArtifactsResource(w http.ResponseWriter, r *http.Request) {
	resource, err := s.deps.Artifacts().Resource(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Artifact resource not found"})
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

func (s *Server) handleFoldersList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Artifacts().ListFolders(r.Context(), stringQueryPtr(r, "parentId"))
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFoldersCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title    string  `json:"title"`
		ParentID *string `json:"parentId"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	folder, err := s.deps.Artifacts().CreateFolder(r.Context(), body.Title, body.ParentID)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		var requestErr artifactservice.BadRequest
		if errors.As(err, &requestErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, folder)
}

func (s *Server) handleFoldersGet(w http.ResponseWriter, r *http.Request) {
	folder, err := s.deps.Artifacts().GetFolder(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, folder)
}

func (s *Server) handleFoldersBreadcrumbs(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Artifacts().FolderBreadcrumbs(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Folder not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func stringQueryPtr(r *http.Request, key string) *string {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil
	}
	return &value
}

func stringFormPtr(r *http.Request, key string) *string {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return nil
	}
	return &value
}
