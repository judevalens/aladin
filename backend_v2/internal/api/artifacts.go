package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	artifactservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerArtifactRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/artifacts/", s.handleArtifactsList)
	mux.HandleFunc("POST /api/artifacts/", s.handleArtifactsCreate)
	mux.HandleFunc("GET /api/artifacts/{id}", s.handleArtifactsGet)
	mux.HandleFunc("PATCH /api/artifacts/{id}", s.handleArtifactsUpdate)
	mux.HandleFunc("DELETE /api/artifacts/{id}", s.handleArtifactsDelete)
	mux.HandleFunc("POST /api/notes/", s.handleNotesCreate)
	mux.HandleFunc("PATCH /api/notes/{id}", s.handleNotesUpdate)
	mux.HandleFunc("DELETE /api/notes/{id}", s.handleNotesDelete)
	mux.HandleFunc("GET /api/notes/{id}/related", s.handleNotesRelated)
	mux.HandleFunc("POST /api/documents/upload", s.handleDocumentsUpload)
	mux.HandleFunc("GET /api/documents/", s.handleDocumentsList)
	mux.HandleFunc("GET /api/documents/{id}/file", s.handleDocumentFile)
	mux.HandleFunc("POST /api/audio/upload", s.handleAudioUpload)
	mux.HandleFunc("GET /api/audio/{filename}", s.handleAudioFile)
}

func (s *Server) handleArtifactsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Artifacts().List(r.Context())
	if err != nil {
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
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Artifact not found"})
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

func (s *Server) handleNotesCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label   string `json:"label"`
		Content string `json:"content"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	rec, err := s.deps.Artifacts().CreateNote(r.Context(), body.Label, body.Content)
	if err != nil {
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

func (s *Server) handleNotesUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label   *string `json:"label"`
		Content *string `json:"content"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	rec, err := s.deps.Artifacts().UpdateNote(r.Context(), r.PathValue("id"), body.Label, body.Content)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Note not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleNotesDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Artifacts().DeleteNote(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Note not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleNotesRelated(w http.ResponseWriter, r *http.Request) {
	limit := min(intQuery(r, "limit", 8), 30)
	out, err := s.deps.Artifacts().RelatedNotes(r.Context(), r.PathValue("id"), limit)
	if err != nil {
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Note not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDocumentsUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only PDF files are supported"})
		return
	}
	bytes, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read upload"})
		return
	}
	rec, err := s.deps.Artifacts().UploadDocument(r.Context(), header.Filename, bytes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleDocumentsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Artifacts().ListDocuments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDocumentFile(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("id")
	if strings.TrimSpace(docID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document id is required"})
		return
	}
	path, err := s.deps.Artifacts().DocumentFilePath(docID)
	if errors.Is(err, artifactservice.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) handleAudioUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()
	rec, err := s.deps.Artifacts().UploadAudio(r.Context(), header.Filename, file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleAudioFile(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	path, err := s.deps.Artifacts().AudioFilePath(filename)
	if errors.Is(err, artifactservice.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeFile(w, r, path)
}
