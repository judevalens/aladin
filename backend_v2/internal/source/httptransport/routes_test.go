package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/source"
)

type sourceServiceStub struct {
	created source.SourceCreateInput
	deleted string
}

func (*sourceServiceStub) List(context.Context) ([]source.SourceRecord, error) {
	return []source.SourceRecord{{ID: "s1", Name: "HN"}}, nil
}
func (s *sourceServiceStub) Create(_ context.Context, input source.SourceCreateInput) (source.SourceRecord, error) {
	s.created = input
	return source.SourceRecord{ID: "s1", Name: input.Name}, nil
}
func (s *sourceServiceStub) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func TestRegisterPreservesSourceRoutes(t *testing.T) {
	service := &sourceServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodGet, "/api/sources/", "", http.StatusOK, `[{"id":"s1"`},
		{http.MethodPost, "/api/sources/", `{"kind":"hackernews_feed","name":"HN"}`, http.StatusCreated, `"name":"HN"`},
		{http.MethodDelete, "/api/sources/20a0ef1a-a0c0-48bb-a778-71d913e50b7f", "", http.StatusOK, `"ok":true`},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if service.created.Kind != "hackernews_feed" || service.deleted == "" {
		t.Fatalf("service calls not preserved: created=%+v deleted=%q", service.created, service.deleted)
	}
}
