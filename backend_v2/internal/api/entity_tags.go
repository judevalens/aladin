package api

import (
	"errors"
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/httpapi"
	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerEntityTagRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/entities/search", s.handleEntitySearch)
	mux.HandleFunc("POST /api/entities", s.handleEntityCreate)
	mux.HandleFunc("GET /api/artifacts/{id}/entities", s.handleArtifactEntitiesList)
	mux.HandleFunc("POST /api/artifacts/{id}/entities", s.handleArtifactEntitiesAttach)
	mux.HandleFunc("DELETE /api/artifacts/{id}/entities/{entityId}", s.handleArtifactEntitiesDetach)
	mux.HandleFunc("PUT /api/artifacts/{id}/entity-mentions", s.handleArtifactMentionsSync)
}

func principalUserID(r *http.Request) string {
	return httpapi.PrincipalUserID(r)
}

// GET /api/entities/search?q=…&limit=… — typeahead for the tag / @entity picker.
func (s *Server) handleEntitySearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	hits, err := s.deps.EntityTags().Search(r.Context(), principalUserID(r), q, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}

// POST /api/entities {name, kind} — mint a new canonical entity ("create new" path).
func (s *Server) handleEntityCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	hit, err := s.deps.EntityTags().CreateEntity(r.Context(), coreservice.CreateEntityInput{Name: payload.Name, Kind: payload.Kind})
	if err != nil {
		if errors.Is(err, coreservice.ErrInvalidEntityTag) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "entity name is required", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusCreated, hit)
}

// GET /api/artifacts/{id}/entities — entities linked to an artifact (tags + mentions).
func (s *Server) handleArtifactEntitiesList(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.EntityTags().ListForArtifact(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/artifacts/{id}/entities {entityId} — tag an artifact with an entity.
func (s *Server) handleArtifactEntitiesAttach(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		EntityID string `json:"entityId"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	err := s.deps.EntityTags().Attach(r.Context(), coreservice.AttachEntityInput{
		ArtifactID: r.PathValue("id"),
		EntityID:   payload.EntityID,
		AddedBy:    principalUserID(r),
	})
	if err != nil {
		if errors.Is(err, coreservice.ErrInvalidEntityTag) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "entityId is required", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/artifacts/{id}/entity-mentions {mentions:[…]} — reconcile projected @entity
// mentions for a page (the set replaces all existing mention rows).
func (s *Server) handleArtifactMentionsSync(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Mentions []coreservice.MentionRef `json:"mentions"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.EntityTags().SyncMentions(r.Context(), r.PathValue("id"), payload.Mentions); err != nil {
		if errors.Is(err, coreservice.ErrInvalidEntityTag) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "artifact id is required", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/artifacts/{id}/entities/{entityId} — remove a tag.
func (s *Server) handleArtifactEntitiesDetach(w http.ResponseWriter, r *http.Request) {
	err := s.deps.EntityTags().Detach(r.Context(), r.PathValue("id"), r.PathValue("entityId"))
	if err != nil {
		if errors.Is(err, coreservice.ErrInvalidEntityTag) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "artifact id and entity id are required", err)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
