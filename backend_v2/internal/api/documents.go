package api

import (
	"errors"
	"net/http"
	"strconv"

	artifactservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerDocumentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/artifacts/{id}/document", s.handleArtifactDocument)
	mux.HandleFunc("GET /api/artifacts/{id}/document/pages", s.handleArtifactDocumentPages)
	mux.HandleFunc("GET /api/artifacts/{id}/outline", s.handleArtifactOutline)
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

// maxDocumentPageRange bounds one pages request — a board's doc window reads a page or
// two at a time; anything wanting the whole book should say so via document?text=1.
const maxDocumentPageRange = 20

// handleArtifactDocumentPages serves a page RANGE of an artifact's ingested text —
// what a board's live doc window reads as its own page turns, without pulling the
// megabytes the full document?text=1 form carries.
//
//	GET /api/artifacts/{id}/document/pages?from=94&to=94
func (s *Server) handleArtifactDocumentPages(w http.ResponseWriter, r *http.Request) {
	from, errFrom := strconv.Atoi(r.URL.Query().Get("from"))
	to, errTo := strconv.Atoi(r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil || from < 1 || to < from {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest,
			"from and to must be positive integers with from <= to", nil)
		return
	}
	if to-from+1 > maxDocumentPageRange {
		to = from + maxDocumentPageRange - 1
	}

	pages, err := s.deps.Documents().Pages(r.Context(), r.PathValue("id"), from, to)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "No ingested document for this artifact", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages})
}

// handleArtifactOutline serves the chunk tree — the structure recovered by segmentation,
// which for most PDFs is an outline the file never carried (INGESTION_PRD §11).
//
// Text is omitted: this is for navigating. Reading is read_document / the viewer.
//
//	GET /api/artifacts/{id}/outline
func (s *Server) handleArtifactOutline(w http.ResponseWriter, r *http.Request) {
	tree, err := s.deps.Documents().Outline(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, artifactservice.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "No ingested document for this artifact", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"chunks": tree})
}
