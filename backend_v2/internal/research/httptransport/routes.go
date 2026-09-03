// Package httptransport owns the public HTTP adapter for Research.
package httptransport

import (
	"errors"
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/research"
	coreservice "aladin/backend_v2/internal/service"
)

type routes struct{ service research.ResearchService }

// Register installs the existing Research routes without changing their public contracts.
func Register(mux *http.ServeMux, service research.ResearchService) {
	routes := routes{service: service}
	mux.HandleFunc("POST /api/research", routes.handleResearchCreate)
	mux.HandleFunc("GET /api/research/{id}", routes.handleResearchGet)
	mux.HandleFunc("PATCH /api/research/{id}", routes.handleResearchPatch)
}

// researchCreateRequest is the wire shape for creating a research folder. Only `title`
// is required — the extension row is created sparse (RESEARCH_SURFACE_PRD §5).
type researchCreateRequest struct {
	ID         string  `json:"id,omitempty"`
	Title      string  `json:"title"`
	ParentID   *string `json:"parentId,omitempty"`
	Hypothesis string  `json:"hypothesis,omitempty"`
}

// handleResearchCreate creates a research folder: the tree node (kind='research') and
// its strategy extension row, in one transaction with the sync frame. The response is
// the committed light node incl. seq, so the caller can apply it locally under the same
// seq guard the WS frame uses.
//
//	POST /api/research
func (routes routes) handleResearchCreate(w http.ResponseWriter, r *http.Request) {
	var req researchCreateRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	node, err := routes.service.Create(r.Context(), research.ResearchCreateInput{
		ID:         req.ID,
		Title:      req.Title,
		ParentID:   req.ParentID,
		Hypothesis: req.Hypothesis,
	})
	if err != nil {
		writeResearchError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, node)
}

// handleResearchGet reads one research folder's strategy facts — the fields the tree
// frame deliberately leaves off (hypothesis, source, hashes), fetched when the Overview
// opens rather than relayed to every tree row.
//
//	GET /api/research/{id}
func (routes routes) handleResearchGet(w http.ResponseWriter, r *http.Request) {
	out, err := routes.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeResearchError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// researchPatchRequest is the research folder's mutable surface. Title only for now;
// the Overview's other fields join it here rather than earning new endpoints.
type researchPatchRequest struct {
	Title      *string `json:"title,omitempty"`
	Hypothesis *string `json:"hypothesis,omitempty"`
}

// handleResearchPatch updates a research folder (title and/or hypothesis). The node's
// upsert frame is emitted in the same transaction, so open surfaces refresh through the
// syncer — including the Overview, whose heavier fields don't ride the frame.
//
//	PATCH /api/research/{id}
func (routes routes) handleResearchPatch(w http.ResponseWriter, r *http.Request) {
	var req researchPatchRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	node, err := routes.service.Update(r.Context(), r.PathValue("id"), research.ResearchPatch{
		Title:      req.Title,
		Hypothesis: req.Hypothesis,
	})
	if err != nil {
		writeResearchError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, node)
}

func writeResearchError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, coreservice.ErrUnauthenticated) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
		return
	}
	if errors.Is(err, coreservice.ErrForbidden) {
		httpapi.WriteError(w, r, http.StatusForbidden, "bad_request", "Forbidden", err)
		return
	}
	if errors.Is(err, coreservice.ErrNotFound) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Research folder not found", err)
		return
	}
	var requestErr coreservice.BadRequest
	if errors.As(err, &requestErr) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
}
