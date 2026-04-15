package api

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *Server) handleAudioUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()

	rec, err := s.deps.Audio.Upload(r.Context(), header.Filename, file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleAudioFile(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	path, err := s.deps.Audio.FilePath(filename)
	if errors.Is(err, ErrNotFound) {
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

func audioDir() string {
	return filepath.Join(".", "audio")
}
