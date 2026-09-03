// Package httptransport owns the public HTTP adapter for ingested Documents.
package httptransport

import (
	"errors"
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/document"
	"aladin/backend_v2/internal/httpapi"
	coreservice "aladin/backend_v2/internal/service"
)

type routes struct{ service document.DocumentService }

// Register installs the existing document routes without changing their contracts.
func Register(mux *http.ServeMux, service document.DocumentService) {
	routes := routes{service: service}
	mux.HandleFunc("GET /api/artifacts/{id}/document", routes.handleArtifactDocument)
	mux.HandleFunc("GET /api/artifacts/{id}/document/pages", routes.handleArtifactDocumentPages)
	mux.HandleFunc("GET /api/artifacts/{id}/outline", routes.handleArtifactOutline)
}

// handleArtifactDocument serves an artifact's ingested document
// (design/INGESTION_PRD.md).
//
//	GET /api/artifacts/{id}/document          → status, page count, outline
//	GET /api/artifacts/{id}/document?text=1   → the above plus the page text
//
// Text is opt-in because a book is megabytes: a Material row wants to know whether the
// document is readable, and only the viewer wants the words.
func (routes routes) handleArtifactDocument(w http.ResponseWriter, r *http.Request) {
	withText := r.URL.Query().Get("text") != ""

	doc, err := routes.service.Get(r.Context(), r.PathValue("id"), withText)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, coreservice.ErrNotFound) {
			// Not an error condition worth alarming about: it means nothing has been
			// ingested for this artifact, which is the normal state for a note or a link.
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "No ingested document for this artifact", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, doc)
}

// maxDocumentPageRange bounds one pages request — a board's doc window reads a page or
// two at a time; anything wanting the whole book should say so via document?text=1.
const maxDocumentPageRange = 20

// handleArtifactDocumentPages serves a page RANGE of an artifact's ingested text —
// what a board's live doc window reads as its own page turns, without pulling the
// megabytes the full document?text=1 form carries.
//
//	GET /api/artifacts/{id}/document/pages?from=94&to=94
func (routes routes) handleArtifactDocumentPages(w http.ResponseWriter, r *http.Request) {
	from, errFrom := strconv.Atoi(r.URL.Query().Get("from"))
	to, errTo := strconv.Atoi(r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil || from < 1 || to < from {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request",
			"from and to must be positive integers with from <= to", nil)
		return
	}
	if to-from+1 > maxDocumentPageRange {
		to = from + maxDocumentPageRange - 1
	}

	pages, err := routes.service.Pages(r.Context(), r.PathValue("id"), from, to)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, coreservice.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "No ingested document for this artifact", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"pages": pages})
}

// handleArtifactOutline serves the chunk tree — the structure recovered by segmentation,
// which for most PDFs is an outline the file never carried (INGESTION_PRD §11).
//
// Text is omitted: this is for navigating. Reading is read_document / the viewer.
//
//	GET /api/artifacts/{id}/outline
func (routes routes) handleArtifactOutline(w http.ResponseWriter, r *http.Request) {
	tree, err := routes.service.Outline(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, coreservice.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "No ingested document for this artifact", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"chunks": tree})
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
