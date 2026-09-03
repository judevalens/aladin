package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/unfurl"
)

type unfurlStub struct{ rawURL string }

func (s *unfurlStub) Unfurl(_ context.Context, rawURL string) (unfurl.Unfurl, error) {
	s.rawURL = rawURL
	return unfurl.Unfurl{URL: rawURL, Title: "Example"}, nil
}

func TestRegisterPreservesUnfurlRoute(t *testing.T) {
	service := &unfurlStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/unfurl", strings.NewReader(`{"url":"https://example.com/article"}`)))
	if res.Code != http.StatusOK || service.rawURL != "https://example.com/article" {
		t.Fatalf("status=%d url=%q body=%s", res.Code, service.rawURL, res.Body.String())
	}
}
