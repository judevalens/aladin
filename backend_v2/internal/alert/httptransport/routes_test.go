package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/alert"
	coreservice "aladin/backend_v2/internal/service"
)

type alertServiceStub struct {
	created alert.CreateAlertResult
	listed  []alert.Alert
}

func (s alertServiceStub) Create(context.Context, string, string, string, float64) (alert.CreateAlertResult, error) {
	return s.created, nil
}
func (s alertServiceStub) List(context.Context, string) ([]alert.Alert, error) { return s.listed, nil }
func (alertServiceStub) Delete(context.Context, string, string) error          { return nil }
func (alertServiceStub) Pause(context.Context, string, string) error           { return nil }

type notificationServiceStub struct{ listed []alert.Notification }

func (notificationServiceStub) Create(context.Context, alert.Notification) (alert.Notification, error) {
	return alert.Notification{}, nil
}
func (s notificationServiceStub) List(context.Context, string, int) ([]alert.Notification, error) {
	return s.listed, nil
}
func (s notificationServiceStub) ListUnread(context.Context, string) ([]alert.Notification, error) {
	return s.listed, nil
}
func (notificationServiceStub) MarkRead(context.Context, string, string) error { return nil }

func TestRegisterPreservesAlertAndNotificationRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux,
		alertServiceStub{
			created: alert.CreateAlertResult{Alert: alert.Alert{ID: "a1", Symbol: "AAPL"}},
			listed:  []alert.Alert{{ID: "a1", Symbol: "AAPL"}},
		},
		notificationServiceStub{listed: []alert.Notification{{ID: "n1", Kind: "price_alert"}}},
	)

	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodGet, "/api/alerts", "", http.StatusOK, `"alerts":[{"id":"a1"`},
		{http.MethodPost, "/api/alerts", `{"symbol":"AAPL","direction":"above","threshold":200}`, http.StatusCreated, `"alert":{"id":"a1"`},
		{http.MethodDelete, "/api/alerts/a1", "", http.StatusOK, `"ok":true`},
		{http.MethodPost, "/api/alerts/a1/pause", "", http.StatusOK, `"ok":true`},
		{http.MethodGet, "/api/notifications", "", http.StatusOK, `"notifications":[{"id":"n1"`},
		{http.MethodPost, "/api/notifications/n1/read", "", http.StatusOK, `"ok":true`},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req = req.WithContext(coreservice.WithPrincipal(req.Context(), coreservice.Principal{UserID: "u1"}))
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.status, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), tc.want) {
				t.Fatalf("body = %s, want substring %s", response.Body.String(), tc.want)
			}
		})
	}
}
