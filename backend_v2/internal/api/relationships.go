package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerRelationshipRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/relationships", s.handleRelationshipCreate)
	mux.HandleFunc("GET /api/relationships", s.handleRelationshipList)
	mux.HandleFunc("DELETE /api/relationships/{id}", s.handleRelationshipDelete)
}

func (s *Server) handleRelationshipCreate(w http.ResponseWriter, r *http.Request) {
	var rel coreservice.Relationship
	if err := readJSON(r, &rel); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid json body", err)
		return
	}
	out, err := s.deps.Relationships().Create(r.Context(), rel)
	if err != nil {
		s.writeRelationshipError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleRelationshipList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.Relationships().ListForNode(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("id"))
	if err != nil {
		s.writeRelationshipError(w, r, err)
		return
	}
	if out == nil {
		out = []coreservice.Relationship{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRelationshipDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Relationships().Delete(r.Context(), r.PathValue("id")); err != nil {
		s.writeRelationshipError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// writeRelationshipError maps a service.BadRequest to 400, anything else to 500.
func (s *Server) writeRelationshipError(w http.ResponseWriter, r *http.Request, err error) {
	var badReq coreservice.BadRequest
	if errors.As(err, &badReq) {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
		return
	}
	writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
}
