package api

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

type documentRecord struct {
	ID         string  `json:"id"`
	Filename   string  `json:"filename"`
	Title      *string `json:"title"`
	Size       *int64  `json:"size,omitempty"`
	PageCount  *int    `json:"page_count"`
	UploadedAt string  `json:"uploaded_at"`
	Ingested   bool    `json:"ingested"`
	Status     string  `json:"status,omitempty"`
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
	rec, err := s.deps.Documents.Upload(r.Context(), header.Filename, bytes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleDocumentsList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Documents.List(r.Context())
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
	path, err := s.deps.Documents.FilePath(docID)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	http.ServeFile(w, r, path)
}

func uploadDir() string {
	return filepath.Join(".", "uploads")
}
