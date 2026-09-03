package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/insights"
)

type insightServiceStub struct {
	params insights.InsightListParams
	status string
}

func (s *insightServiceStub) List(_ context.Context, params insights.InsightListParams) (map[string]any, error) {
	s.params = params
	return map[string]any{"items": []insights.InsightRecord{{ID: "i1"}}, "total": 1}, nil
}
func (*insightServiceStub) Stats(context.Context) (map[string]any, error) {
	return map[string]any{"byType": map[string]int{"trend": 1}}, nil
}
func (s *insightServiceStub) UpdateStatus(_ context.Context, _ string, status string) error {
	s.status = status
	return nil
}

func TestRegisterPreservesInsightRoutes(t *testing.T) {
	service := &insightServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/insights/?limit=4&offset=2&type=trend&status=accepted", `"items":[{"id":"i1"`},
		{http.MethodGet, "/api/insights/stats", `"byType"`},
		{http.MethodPost, "/api/insights/i1/accept", `"ok":true`},
		{http.MethodPost, "/api/insights/i1/dismiss", `"ok":true`},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if service.params.Limit != 4 || service.params.Offset != 2 || service.params.Type != "trend" || service.params.Status != "accepted" {
		t.Fatalf("list params changed: %+v", service.params)
	}
	if service.status != "dismissed" {
		t.Fatalf("last status = %q", service.status)
	}
}
