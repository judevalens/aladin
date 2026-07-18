package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerEntityContextRoutes(mux *http.ServeMux) {
	// The index. ServeMux prefers the more specific /api/entities/search (registered by
	// the tag routes), so a bare GET here doesn't shadow it.
	mux.HandleFunc("GET /api/entities", s.handleEntityList)
	mux.HandleFunc("GET /api/entities/merge-queue", s.handleMergeQueue)
	mux.HandleFunc("GET /api/entities/{id}/context", s.handleEntityContext)
	mux.HandleFunc("POST /api/entities/{id}/edges", s.handleEntityDrawEdge)
	mux.HandleFunc("POST /api/entities/{id}/merges/{mergeId}/accept", s.handleEntityMergeAccept)
	mux.HandleFunc("POST /api/entities/{id}/merges/{mergeId}/reject", s.handleEntityMergeReject)
}

// GET /api/entities?q=&kind=&sort=&limit= — the Entities index. An empty q lists the
// registry (unlike /api/entities/search, which is a typeahead and returns nothing).
func (s *Server) handleEntityList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.deps.EntityList().List(r.Context(), coreservice.EntityListQuery{
		OwnerUserID: principalUserID(r),
		Query:       r.URL.Query().Get("q"),
		Kind:        r.URL.Query().Get("kind"),
		Filter:      r.URL.Query().Get("filter"),
		Sort:        r.URL.Query().Get("sort"),
		Limit:       limit,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/entities/merge-queue?limit= — the inbox: all pending merge decisions.
func (s *Server) handleMergeQueue(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.deps.EntityContext().MergeQueue(r.Context(), limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/entities/{id}/merges/{mergeId}/accept — fold the two entities together.
// Reversible: ids stay resolvable through the overlay.
func (s *Server) handleEntityMergeAccept(w http.ResponseWriter, r *http.Request) {
	s.decideMerge(w, r, s.deps.EntityContext().AcceptMerge)
}

// POST /api/entities/{id}/merges/{mergeId}/reject — they're distinct. Recorded as
// negative evidence so the pair is never proposed again.
func (s *Server) handleEntityMergeReject(w http.ResponseWriter, r *http.Request) {
	s.decideMerge(w, r, s.deps.EntityContext().RejectMerge)
}

func (s *Server) decideMerge(w http.ResponseWriter, r *http.Request, apply func(context.Context, string) error) {
	err := apply(r.Context(), r.PathValue("mergeId"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, coreservice.ErrInvalidEntityTag):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid merge id", err)
	case errors.Is(err, coreservice.ErrMergeNotPending):
		// Already decided (or gone) — e.g. a double-click, or the judge got there first.
		writeAPIError(w, r, http.StatusConflict, categoryBadRequest, "merge already decided", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}

// GET /api/entities/{id}/context — the Entity Context surface payload: identity, typed
// edges, and the verbatim accreted material. A merged-away id resolves to its canonical
// root rather than 404ing.
func (s *Server) handleEntityContext(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.EntityContext().Get(r.Context(), principalUserID(r), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, coreservice.ErrInvalidEntityTag) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid entity id", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/entities/{id}/edges {rel, toId, why} — draw a connection. Writes a
// user-authored edge (origin 'you' → the YOURS tag). Idempotent per (user, from, to, rel).
func (s *Server) handleEntityDrawEdge(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Rel  string `json:"rel"`
		ToID string `json:"toId"`
		Why  string `json:"why"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	err := s.deps.EntityContext().DrawEdge(r.Context(), coreservice.DrawEdgeInput{
		OwnerUserID: principalUserID(r),
		FromID:      r.PathValue("id"),
		ToID:        payload.ToID,
		Rel:         payload.Rel,
		Why:         payload.Why,
	})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, coreservice.ErrInvalidEntityTag):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid edge", err)
	case errors.Is(err, coreservice.ErrEntityNotFound):
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "entity not found", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}
