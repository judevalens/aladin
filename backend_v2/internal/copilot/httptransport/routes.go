package httptransport

import (
	"errors"
	"net/http"
	"strings"

	"aladin/backend_v2/internal/copilot"
	"aladin/backend_v2/internal/httpapi"
	coreservice "aladin/backend_v2/internal/service"
)

type routes struct{ service copilot.CopilotService }

func Register(mux *http.ServeMux, service copilot.CopilotService) {
	r := routes{service: service}
	mux.HandleFunc("POST /api/copilot/message", r.handleCopilotMessage)
	mux.HandleFunc("POST /api/copilot/cancel", r.handleCopilotCancel)
	mux.HandleFunc("POST /api/copilot/action/{id}/approve", r.handleCopilotApprove)
	mux.HandleFunc("POST /api/copilot/action/{id}/reject", r.handleCopilotReject)
	mux.HandleFunc("GET /api/copilot/threads", r.handleCopilotThreads)
	mux.HandleFunc("GET /api/copilot/threads/{id}", r.handleCopilotThread)
	mux.HandleFunc("PATCH /api/copilot/threads/{id}", r.handleCopilotRenameThread)
	mux.HandleFunc("DELETE /api/copilot/threads/{id}", r.handleCopilotArchiveThread)
	mux.HandleFunc("POST /api/copilot/threads/{id}/pin", r.handleCopilotPinThread)
	mux.HandleFunc("GET /api/copilot/status", r.handleCopilotStatus)
}

// POST /api/copilot/action/{id}/approve — run a proposed destructive action.
func (h routes) handleCopilotApprove(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ApproveAction(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id")); err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/copilot/action/{id}/reject — discard a proposed action.
func (h routes) handleCopilotReject(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RejectAction(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id")); err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type copilotMessageRequest struct {
	ThreadID string                 `json:"threadId"`
	Text     string                 `json:"text"`
	Model    string                 `json:"model"`
	Effort   string                 `json:"effort"`
	Surface  copilot.CopilotSurface `json:"surface"`
}

// POST /api/copilot/message — start/continue a thread. Persists the user turn, spawns the
// agent loop async, and returns {threadId, sessionId} (202). The answer streams over
// /api/events/ws as copilot.* events.
func (h routes) handleCopilotMessage(w http.ResponseWriter, r *http.Request) {
	principal, ok := coreservice.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	var req copilotMessageRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	result, err := h.service.SendMessage(r.Context(), copilot.CopilotSendInput{
		Principal: principal,
		Bearer:    copilotBearer(r),
		ThreadID:  req.ThreadID,
		Text:      req.Text,
		Model:     req.Model,
		Effort:    req.Effort,
		Surface:   req.Surface,
	})
	if err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, result)
}

// POST /api/copilot/cancel {sessionId} — stop an in-flight turn owned by the caller.
func (h routes) handleCopilotCancel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	if err := h.service.Cancel(r.Context(), httpapi.PrincipalUserID(r), req.SessionID); err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/copilot/status — preflight health for the dock (sidecar + MCP tool server).
func (h routes) handleCopilotStatus(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, h.service.Status(r.Context()))
}

// GET /api/copilot/threads — the signed-in user's copilot threads, newest first.
func (h routes) handleCopilotThreads(w http.ResponseWriter, r *http.Request) {
	threads, err := h.service.ListThreads(r.Context(), httpapi.PrincipalUserID(r))
	if err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"threads": threads})
}

// GET /api/copilot/threads/{id} — one thread's full message history.
func (h routes) handleCopilotThread(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.GetThread(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id"))
	if err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, detail)
}

// PATCH /api/copilot/threads/{id} {title} — rename one owned thread.
func (h routes) handleCopilotRenameThread(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	thread, err := h.service.RenameThread(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id"), req.Title)
	if err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, thread)
}

// DELETE /api/copilot/threads/{id} — archive one owned thread (soft delete).
func (h routes) handleCopilotArchiveThread(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ArchiveThread(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id")); err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/copilot/threads/{id}/pin {pinned} — pin or unpin one owned thread.
func (h routes) handleCopilotPinThread(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	thread, err := h.service.SetThreadPinned(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id"), req.Pinned)
	if err != nil {
		h.writeCopilotError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, thread)
}

// copilotBearer extracts the caller's raw credential so the copilot-agent sidecar can
// call the MCP server AS this user (same scoping as the user's own API calls). The
// session cookie is the normal path (the dock is a browser fetch); an Authorization
// bearer covers token-authenticated clients.
func copilotBearer(r *http.Request) string {
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		return strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	}
	if cookie, err := r.Cookie(coreservice.SessionCookieName); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func (h routes) writeCopilotError(w http.ResponseWriter, r *http.Request, err error) {
	if writeAccessError(w, r, err) {
		return
	}
	var badRequest coreservice.BadRequest
	switch {
	case errors.As(err, &badRequest):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
	case errors.Is(err, coreservice.ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Thread not found", err)
	case errors.Is(err, coreservice.ErrConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "bad_request", "The copilot is still answering in this thread — stop it or wait for it to finish", err)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
	}
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
