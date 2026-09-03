package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/feed"
)

type feedServiceStub struct {
	params feed.FeedListParams
	id     string
	status string
}

func (s *feedServiceStub) List(_ context.Context, params feed.FeedListParams) (map[string]any, error) {
	s.params = params
	return map[string]any{"items": []feed.FeedItem{{ID: "r1"}}, "total": 1}, nil
}
func (*feedServiceStub) Topics(context.Context) ([]string, error) {
	return []string{"markets"}, nil
}
func (*feedServiceStub) Sources(context.Context) ([]feed.FeedSourceRecord, error) {
	return []feed.FeedSourceRecord{{ID: "s1", Name: "Source"}}, nil
}
func (s *feedServiceStub) UpdateStatus(_ context.Context, id, status string) error {
	s.id, s.status = id, status
	return nil
}

func TestRegisterPreservesFeedRoutes(t *testing.T) {
	service := &feedServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/feed/?limit=5&offset=2&source_type=rss&topic=markets&saved=true&sort=signal", `"items":[{"id":"r1"`},
		{http.MethodGet, "/api/feed/topics", `["markets"]`},
		{http.MethodGet, "/api/feed/sources", `[{"id":"s1"`},
		{http.MethodPost, "/api/feed/r1/save", `"ok":true`},
		{http.MethodPost, "/api/feed/r1/dismiss", `"ok":true`},
		{http.MethodPost, "/api/feed/r1/unsave", `"ok":true`},
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

	if service.params.Limit != 5 || service.params.Offset != 2 || service.params.SourceType != "rss" || service.params.Topic != "markets" || !service.params.SavedOnly || service.params.Sort != "signal" {
		t.Fatalf("list params changed: %+v", service.params)
	}
	if service.id != "r1" || service.status != "" {
		t.Fatalf("last status update = (%q, %q)", service.id, service.status)
	}
}
