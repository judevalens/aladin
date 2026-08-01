package api

import (
	"errors"
	"net/http"

	artifactservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerDocumentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/artifacts/{id}/document", s.handleArtifactDocument)
}

// handleArtifactDocument serves an artifact's ingested document
// (design/INGESTION_PRD.md).
//
//	GET /api/artifacts/{id}/document          → status, page count, outline
//	GET /api/artifacts/{id}/document?text=1   → the above plus the page text
//
// Text is opt-in because a book is megabytes: a Material row wants to know whether the
// document is readable, and only the viewer wants the words.
func (s *Server) handleArtifactDocument(w http.ResponseWriter, r *http.Request) {
	withText := r.URL.Query().Get("text") != ""

	doc, err := s.deps.Documents().Get(r.Context(), r.PathValue("id"), withText)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, artifactservice.ErrNotFound) {
			// Not an error condition worth alarming about: it means nothing has been
			// ingested for this artifact, which is the normal state for a note or a link.
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "No ingested document for this artifact", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}
