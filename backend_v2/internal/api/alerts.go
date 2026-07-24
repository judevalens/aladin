package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerAlertRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/alerts", s.handleAlertsList)
	mux.HandleFunc("POST /api/alerts", s.handleAlertsCreate)
	mux.HandleFunc("DELETE /api/alerts/{id}", s.handleAlertsDelete)
	mux.HandleFunc("POST /api/alerts/{id}/pause", s.handleAlertsPause)

	mux.HandleFunc("GET /api/notifications", s.handleNotificationsList)
	mux.HandleFunc("POST /api/notifications/{id}/read", s.handleNotificationRead)
}

func (s *Server) handleAlertsList(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Alerts().List(r.Context(), principalUserID(r))
	if err != nil {
		s.writeAlertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": items})
}

// POST /api/alerts {symbol, direction, threshold} — create a price alert.
func (s *Server) handleAlertsCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Symbol    string  `json:"symbol"`
		Direction string  `json:"direction"`
		Threshold float64 `json:"threshold"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	res, err := s.deps.Alerts().Create(r.Context(), principalUserID(r), payload.Symbol, payload.Direction, payload.Threshold)
	if err != nil {
		s.writeAlertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) handleAlertsDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Alerts().Delete(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeAlertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAlertsPause(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Alerts().Pause(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeAlertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/notifications[?unread=1] — the durable inbox (the offline-survival read path).
func (s *Server) handleNotificationsList(w http.ResponseWriter, r *http.Request) {
	userID := principalUserID(r)
	var (
		items []coreservice.Notification
		err   error
	)
	if r.URL.Query().Get("unread") == "1" {
		items, err = s.deps.Notifications().ListUnread(r.Context(), userID)
	} else {
		items, err = s.deps.Notifications().List(r.Context(), userID, 0)
	}
	if err != nil {
		s.writeAlertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": items})
}

func (s *Server) handleNotificationRead(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Notifications().MarkRead(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeAlertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) writeAlertError(w http.ResponseWriter, r *http.Request, err error) {
	var badRequest coreservice.BadRequest
	switch {
	case errors.As(err, &badRequest):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	case errors.Is(err, coreservice.ErrUnauthenticated):
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}
