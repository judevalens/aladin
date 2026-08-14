package api

import (
	"errors"
	"net/http"
	"strings"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerCopilotRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/copilot/message", s.handleCopilotMessage)
	mux.HandleFunc("POST /api/copilot/cancel", s.handleCopilotCancel)
	mux.HandleFunc("POST /api/copilot/action/{id}/approve", s.handleCopilotApprove)
	mux.HandleFunc("POST /api/copilot/action/{id}/reject", s.handleCopilotReject)
	mux.HandleFunc("GET /api/copilot/threads", s.handleCopilotThreads)
	mux.HandleFunc("GET /api/copilot/threads/{id}", s.handleCopilotThread)
	mux.HandleFunc("PATCH /api/copilot/threads/{id}", s.handleCopilotRenameThread)
	mux.HandleFunc("DELETE /api/copilot/threads/{id}", s.handleCopilotArchiveThread)
	mux.HandleFunc("POST /api/copilot/threads/{id}/pin", s.handleCopilotPinThread)
	mux.HandleFunc("GET /api/copilot/status", s.handleCopilotStatus)
}

// POST /api/copilot/action/{id}/approve — run a proposed destructive action.
func (s *Server) handleCopilotApprove(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Copilot().ApproveAction(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/copilot/action/{id}/reject — discard a proposed action.
func (s *Server) handleCopilotReject(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Copilot().RejectAction(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type copilotMessageRequest struct {
	ThreadID string                     `json:"threadId"`
	Text     string                     `json:"text"`
	Model    string                     `json:"model"`
	Effort   string                     `json:"effort"`
	Surface  coreservice.CopilotSurface `json:"surface"`
}

// POST /api/copilot/message — start/continue a thread. Persists the user turn, spawns the
// agent loop async, and returns {threadId, sessionId} (202). The answer streams over
// /api/events/ws as copilot.* events.
func (s *Server) handleCopilotMessage(w http.ResponseWriter, r *http.Request) {
	principal, ok := coreservice.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	var req copilotMessageRequest
	if err := readJSON(r, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	result, err := s.deps.Copilot().SendMessage(r.Context(), coreservice.CopilotSendInput{
		Principal: principal,
		Bearer:    copilotBearer(r),
		ThreadID:  req.ThreadID,
		Text:      req.Text,
		Model:     req.Model,
		Effort:    req.Effort,
		Surface:   req.Surface,
	})
	if err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

// POST /api/copilot/cancel {sessionId} — stop an in-flight turn owned by the caller.
func (s *Server) handleCopilotCancel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.Copilot().Cancel(r.Context(), principalUserID(r), req.SessionID); err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/copilot/status — preflight health for the dock (sidecar + MCP tool server).
func (s *Server) handleCopilotStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Copilot().Status(r.Context()))
}

// GET /api/copilot/threads — the signed-in user's copilot threads, newest first.
func (s *Server) handleCopilotThreads(w http.ResponseWriter, r *http.Request) {
	threads, err := s.deps.Copilot().ListThreads(r.Context(), principalUserID(r))
	if err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"threads": threads})
}

// GET /api/copilot/threads/{id} — one thread's full message history.
func (s *Server) handleCopilotThread(w http.ResponseWriter, r *http.Request) {
	detail, err := s.deps.Copilot().GetThread(r.Context(), principalUserID(r), r.PathValue("id"))
	if err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// PATCH /api/copilot/threads/{id} {title} — rename one owned thread.
func (s *Server) handleCopilotRenameThread(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := readJSON(r, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	thread, err := s.deps.Copilot().RenameThread(r.Context(), principalUserID(r), r.PathValue("id"), req.Title)
	if err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

// DELETE /api/copilot/threads/{id} — archive one owned thread (soft delete).
func (s *Server) handleCopilotArchiveThread(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Copilot().ArchiveThread(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/copilot/threads/{id}/pin {pinned} — pin or unpin one owned thread.
func (s *Server) handleCopilotPinThread(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := readJSON(r, &req); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	thread, err := s.deps.Copilot().SetThreadPinned(r.Context(), principalUserID(r), r.PathValue("id"), req.Pinned)
	if err != nil {
		s.writeCopilotError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, thread)
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

func (s *Server) writeCopilotError(w http.ResponseWriter, r *http.Request, err error) {
	if writeAccessError(w, r, err) {
		return
	}
	var badRequest coreservice.BadRequest
	switch {
	case errors.As(err, &badRequest):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	case errors.Is(err, coreservice.ErrNotFound):
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Thread not found", err)
	case errors.Is(err, coreservice.ErrConflict):
		writeAPIError(w, r, http.StatusConflict, categoryBadRequest, "The copilot is still answering in this thread — stop it or wait for it to finish", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}
