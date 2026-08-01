package api

import (
	"errors"
	"net/http"

	artifactservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerResearchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/research", s.handleResearchCreate)
	mux.HandleFunc("GET /api/research/{id}", s.handleResearchGet)
	mux.HandleFunc("PATCH /api/research/{id}", s.handleResearchPatch)
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
func (s *Server) handleResearchCreate(w http.ResponseWriter, r *http.Request) {
	var req researchCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	node, err := s.deps.Research().Create(r.Context(), artifactservice.ResearchCreateInput{
		ID:         req.ID,
		Title:      req.Title,
		ParentID:   req.ParentID,
		Hypothesis: req.Hypothesis,
	})
	if err != nil {
		writeResearchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

// handleResearchGet reads one research folder's strategy facts — the fields the tree
// frame deliberately leaves off (hypothesis, source, hashes), fetched when the Overview
// opens rather than relayed to every tree row.
//
//	GET /api/research/{id}
func (s *Server) handleResearchGet(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Research().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeResearchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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
func (s *Server) handleResearchPatch(w http.ResponseWriter, r *http.Request) {
	var req researchPatchRequest
	if err := readJSON(r, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	node, err := s.deps.Research().Update(r.Context(), r.PathValue("id"), artifactservice.ResearchPatch{
		Title:      req.Title,
		Hypothesis: req.Hypothesis,
	})
	if err != nil {
		writeResearchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func writeResearchError(w http.ResponseWriter, r *http.Request, err error) {
	if writeAccessError(w, r, err) {
		return
	}
	if errors.Is(err, artifactservice.ErrNotFound) {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Research folder not found", err)
		return
	}
	var requestErr artifactservice.BadRequest
	if errors.As(err, &requestErr) {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
		return
	}
	writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
}
