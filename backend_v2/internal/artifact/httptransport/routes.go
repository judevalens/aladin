package httptransport

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/httpapi"
)

const (
	categoryBadRequest   = "bad_request"
	categoryNotFound     = "not_found"
	categoryServiceError = "service_error"
)

type routes struct {
	service artifact.ArtifactService
}

func Register(mux *http.ServeMux, service artifact.ArtifactService) {
	h := routes{service: service}
	mux.HandleFunc("GET /api/browser/tree", h.handleBrowserTree)
	mux.HandleFunc("POST /api/browser/nodes", h.handleBrowserNodesCreate)
	mux.HandleFunc("DELETE /api/browser/nodes/{id}", h.handleBrowserNodesDelete)
	mux.HandleFunc("GET /api/artifacts/", h.handleArtifactsList)
	mux.HandleFunc("POST /api/artifacts/", h.handleArtifactsCreate)
	mux.HandleFunc("POST /api/artifacts/upload", h.handleArtifactsUpload)
	// Literal segments beat the {id} wildcard in Go's mux, so these don't shadow the get-by-id.
	mux.HandleFunc("GET /api/artifacts/query", h.handleArtifactsQueryByProperty)
	mux.HandleFunc("GET /api/artifacts/property-facets", h.handleArtifactPropertyFacets)
	mux.HandleFunc("GET /api/artifacts/{id}", h.handleArtifactsGet)
	mux.HandleFunc("PATCH /api/artifacts/{id}", h.handleArtifactsUpdate)
	mux.HandleFunc("DELETE /api/artifacts/{id}", h.handleArtifactsDelete)
	mux.HandleFunc("GET /api/artifacts/{id}/resource", h.handleArtifactsResource)

	mux.HandleFunc("GET /api/folders/", h.handleFoldersList)
	mux.HandleFunc("GET /api/folders/tree", h.handleFoldersTree)
	mux.HandleFunc("POST /api/folders/", h.handleFoldersCreate)
	mux.HandleFunc("GET /api/folders/{id}", h.handleFoldersGet)
	mux.HandleFunc("PATCH /api/folders/{id}", h.handleFoldersUpdate)
	mux.HandleFunc("GET /api/folders/{id}/breadcrumbs", h.handleFoldersBreadcrumbs)
}

func (h routes) handleBrowserNodesCreate(w http.ResponseWriter, r *http.Request) {
	var input artifact.BrowserNodeCreateInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	rec, err := h.service.CreateBrowserNode(r.Context(), input)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, rec)
}

func (h routes) handleBrowserTree(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.BrowserTree(r.Context())
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handleBrowserNodesDelete(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.DeleteBrowserNode(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Browser node not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h routes) handleArtifactsList(w http.ResponseWriter, r *http.Request) {
	params := artifact.ArtifactListParams{
		FolderID: stringQueryPtr(r, "folderId"),
	}
	out, err := h.service.List(r.Context(), params)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handleArtifactsGet(w http.ResponseWriter, r *http.Request) {
	rec, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Artifact not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, rec)
}

func (h routes) handleArtifactsCreate(w http.ResponseWriter, r *http.Request) {
	var payload artifact.ArtifactPayload
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	rec, err := h.service.Create(r.Context(), payload)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, rec)
}

func (h routes) handleArtifactsUpdate(w http.ResponseWriter, r *http.Request) {
	var patch artifact.ArtifactPatch
	if err := httpapi.ReadJSON(r, &patch); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	rec, err := h.service.Update(r.Context(), r.PathValue("id"), patch)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Artifact or folder not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, rec)
}

func (h routes) handleArtifactsDelete(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Artifact not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h routes) handleArtifactsUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, "no file provided", err)
		return
	}
	defer file.Close()

	typeValue := strings.TrimSpace(r.FormValue("type"))
	title := stringFormPtr(r, "title")
	summary := stringFormPtr(r, "summary")
	folderID := stringFormPtr(r, "folderId")
	rec, err := h.service.Upload(r.Context(), artifact.ArtifactUploadInput{
		Type:        typeValue,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Title:       title,
		Summary:     summary,
		FolderID:    folderID,
	}, file)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, rec)
}

func (h routes) handleArtifactsResource(w http.ResponseWriter, r *http.Request) {
	resource, err := h.service.Resource(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Artifact resource not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	if resource.ContentType == "" {
		if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(resource.Path))); contentType != "" {
			resource.ContentType = contentType
		}
	}
	if resource.ContentType != "" {
		w.Header().Set("Content-Type", resource.ContentType)
	}
	http.ServeFile(w, r, resource.Path)
}

func (h routes) handleFoldersList(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.ListFolders(r.Context(), stringQueryPtr(r, "parentId"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handleFoldersTree(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.FolderTree(r.Context())
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handleFoldersCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title    string  `json:"title"`
		ParentID *string `json:"parentId"`
	}
	if err := httpapi.ReadJSON(r, &body); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	folder, err := h.service.CreateFolder(r.Context(), body.Title, body.ParentID)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, folder)
}

func (h routes) handleFoldersGet(w http.ResponseWriter, r *http.Request) {
	folder, err := h.service.GetFolder(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, folder)
}

func (h routes) handleFoldersUpdate(w http.ResponseWriter, r *http.Request) {
	var patch artifact.FolderPatch
	if err := httpapi.ReadJSON(r, &patch); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	folder, err := h.service.UpdateFolder(r.Context(), r.PathValue("id"), patch)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, folder)
}

func (h routes) handleFoldersBreadcrumbs(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.FolderBreadcrumbs(r.Context(), r.PathValue("id"))
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		if errors.Is(err, apperror.ErrNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, categoryNotFound, "Folder not found", err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, items)
}

func stringQueryPtr(r *http.Request, key string) *string {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil
	}
	return &value
}

func stringFormPtr(r *http.Request, key string) *string {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return nil
	}
	return &value
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func writeAccessError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", err)
		return true
	case errors.Is(err, auth.ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, categoryBadRequest, "Forbidden", err)
		return true
	default:
		return false
	}
}

// handleArtifactsQueryByProperty is the H1c read: the caller's artifacts filtered by a typed
// property. `value` is optional — omit it to match every artifact carrying the key.
//
//	GET /api/artifacts/query?key=Status&value=Live[&limit=100]
func (h routes) handleArtifactsQueryByProperty(w http.ResponseWriter, r *http.Request) {
	params := artifact.PropertyQuery{
		Key:   r.URL.Query().Get("key"),
		Value: r.URL.Query().Get("value"),
		Limit: intQuery(r, "limit", 0),
	}
	out, err := h.service.QueryByProperty(r.Context(), params)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		var requestErr apperror.BadRequest
		if errors.As(err, &requestErr) {
			httpapi.WriteError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// handleArtifactPropertyFacets lists the property keys/values in use, so a filter UI can offer
// real choices rather than free text.
//
//	GET /api/artifacts/property-facets
func (h routes) handleArtifactPropertyFacets(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.PropertyFacets(r.Context())
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}
