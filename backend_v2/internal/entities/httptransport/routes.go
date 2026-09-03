package httptransport

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/httpapi"
)

type routes struct {
	tags    entities.EntityTagService
	context entities.EntityContextService
	list    entities.EntityListService
}

func Register(mux *http.ServeMux, tags entities.EntityTagService, entityContext entities.EntityContextService, list entities.EntityListService) {
	h := routes{tags: tags, context: entityContext, list: list}
	mux.HandleFunc("GET /api/entities/search", h.search)
	mux.HandleFunc("POST /api/entities", h.create)
	mux.HandleFunc("GET /api/artifacts/{id}/entities", h.listForArtifact)
	mux.HandleFunc("POST /api/artifacts/{id}/entities", h.attach)
	mux.HandleFunc("DELETE /api/artifacts/{id}/entities/{entityId}", h.detach)
	mux.HandleFunc("PUT /api/artifacts/{id}/entity-mentions", h.syncMentions)
	mux.HandleFunc("GET /api/entities", h.listEntities)
	mux.HandleFunc("GET /api/entities/merge-queue", h.mergeQueue)
	mux.HandleFunc("GET /api/entities/{id}/context", h.getContext)
	mux.HandleFunc("POST /api/entities/{id}/edges", h.drawEdge)
	mux.HandleFunc("POST /api/entities/{id}/merges/{mergeId}/accept", h.acceptMerge)
	mux.HandleFunc("POST /api/entities/{id}/merges/{mergeId}/reject", h.rejectMerge)
}

func (h routes) search(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	hits, err := h.tags.Search(r.Context(), httpapi.PrincipalUserID(r), r.URL.Query().Get("q"), limit)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, hits)
}

func (h routes) create(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	hit, err := h.tags.CreateEntity(r.Context(), entities.CreateEntityInput{Name: payload.Name, Kind: payload.Kind})
	if err != nil {
		if errors.Is(err, entities.ErrInvalidEntityTag) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "entity name is required", err)
			return
		}
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, hit)
}

func (h routes) listForArtifact(w http.ResponseWriter, r *http.Request) {
	out, err := h.tags.ListForArtifact(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) attach(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		EntityID string `json:"entityId"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	err := h.tags.Attach(r.Context(), entities.AttachEntityInput{
		ArtifactID: r.PathValue("id"), EntityID: payload.EntityID, AddedBy: httpapi.PrincipalUserID(r),
	})
	if err != nil {
		if errors.Is(err, entities.ErrInvalidEntityTag) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "entityId is required", err)
			return
		}
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h routes) syncMentions(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Mentions []entities.MentionRef `json:"mentions"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	if err := h.tags.SyncMentions(r.Context(), r.PathValue("id"), payload.Mentions); err != nil {
		if errors.Is(err, entities.ErrInvalidEntityTag) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "artifact id is required", err)
			return
		}
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h routes) detach(w http.ResponseWriter, r *http.Request) {
	err := h.tags.Detach(r.Context(), r.PathValue("id"), r.PathValue("entityId"))
	if err != nil {
		if errors.Is(err, entities.ErrInvalidEntityTag) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "artifact id and entity id are required", err)
			return
		}
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h routes) listEntities(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.list.List(r.Context(), entities.EntityListQuery{
		OwnerUserID: httpapi.PrincipalUserID(r), Query: r.URL.Query().Get("q"), Kind: r.URL.Query().Get("kind"),
		Filter: r.URL.Query().Get("filter"), Sort: r.URL.Query().Get("sort"), Limit: limit,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) mergeQueue(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.context.MergeQueue(r.Context(), limit)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) acceptMerge(w http.ResponseWriter, r *http.Request) {
	h.decideMerge(w, r, h.context.AcceptMerge)
}

func (h routes) rejectMerge(w http.ResponseWriter, r *http.Request) {
	h.decideMerge(w, r, h.context.RejectMerge)
}

func (h routes) decideMerge(w http.ResponseWriter, r *http.Request, apply func(context.Context, string) error) {
	err := apply(r.Context(), r.PathValue("mergeId"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, entities.ErrInvalidEntityTag):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid merge id", err)
	case errors.Is(err, entities.ErrMergeNotPending):
		httpapi.WriteError(w, r, http.StatusConflict, "bad_request", "merge already decided", err)
	default:
		writeServiceError(w, r, err)
	}
}

func (h routes) getContext(w http.ResponseWriter, r *http.Request) {
	out, err := h.context.Get(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, entities.ErrInvalidEntityTag) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid entity id", err)
			return
		}
		writeServiceError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) drawEdge(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Rel  string `json:"rel"`
		ToID string `json:"toId"`
		Why  string `json:"why"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	err := h.context.DrawEdge(r.Context(), entities.DrawEdgeInput{
		OwnerUserID: httpapi.PrincipalUserID(r), FromID: r.PathValue("id"), ToID: payload.ToID, Rel: payload.Rel, Why: payload.Why,
	})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, entities.ErrInvalidEntityTag):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid edge", err)
	case errors.Is(err, entities.ErrEntityNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "entity not found", err)
	default:
		writeServiceError(w, r, err)
	}
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
}
