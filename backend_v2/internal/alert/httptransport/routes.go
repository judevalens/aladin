// Package httptransport owns the Alerts domain HTTP adapter.
package httptransport

import (
	"errors"
	"net/http"

	"aladin/backend_v2/internal/alert"
	"aladin/backend_v2/internal/httpapi"
	coreservice "aladin/backend_v2/internal/service"
)

type handler struct {
	alerts        alert.AlertService
	notifications alert.NotificationService
}

// Register mounts the existing alerts and notifications HTTP contract.
func Register(mux *http.ServeMux, alerts alert.AlertService, notifications alert.NotificationService) {
	h := handler{alerts: alerts, notifications: notifications}
	mux.HandleFunc("GET /api/alerts", h.listAlerts)
	mux.HandleFunc("POST /api/alerts", h.createAlert)
	mux.HandleFunc("DELETE /api/alerts/{id}", h.deleteAlert)
	mux.HandleFunc("POST /api/alerts/{id}/pause", h.pauseAlert)
	mux.HandleFunc("GET /api/notifications", h.listNotifications)
	mux.HandleFunc("POST /api/notifications/{id}/read", h.markNotificationRead)
}

func (h handler) listAlerts(w http.ResponseWriter, r *http.Request) {
	items, err := h.alerts.List(r.Context(), httpapi.PrincipalUserID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"alerts": items})
}

func (h handler) createAlert(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Symbol    string  `json:"symbol"`
		Direction string  `json:"direction"`
		Threshold float64 `json:"threshold"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	result, err := h.alerts.Create(r.Context(), httpapi.PrincipalUserID(r), payload.Symbol, payload.Direction, payload.Threshold)
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, result)
}

func (h handler) deleteAlert(w http.ResponseWriter, r *http.Request) {
	if err := h.alerts.Delete(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h handler) pauseAlert(w http.ResponseWriter, r *http.Request) {
	if err := h.alerts.Pause(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	userID := httpapi.PrincipalUserID(r)
	var (
		items []alert.Notification
		err   error
	)
	if r.URL.Query().Get("unread") == "1" {
		items, err = h.notifications.ListUnread(r.Context(), userID)
	} else {
		items, err = h.notifications.List(r.Context(), userID, 0)
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"notifications": items})
}

func (h handler) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	if err := h.notifications.MarkRead(r.Context(), httpapi.PrincipalUserID(r), r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var badRequest coreservice.BadRequest
	switch {
	case errors.As(err, &badRequest):
		httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
	case errors.Is(err, coreservice.ErrUnauthenticated):
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", err)
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
	}
}
