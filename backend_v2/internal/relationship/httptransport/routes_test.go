package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/relationship"
)

type relationshipServiceStub struct{ deleted string }

func (*relationshipServiceStub) Create(_ context.Context, item relationship.Relationship) (relationship.Relationship, error) {
	item.ID = "rel-1"
	return item, nil
}
func (*relationshipServiceStub) ListForNode(context.Context, string, string) ([]relationship.Relationship, error) {
	return []relationship.Relationship{{ID: "rel-1", RelType: "cites"}}, nil
}
func (s *relationshipServiceStub) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func TestRegisterPreservesRelationshipRoutes(t *testing.T) {
	service := &relationshipServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/relationships", `{"srcKind":"artifact","srcId":"a1","dstKind":"record","dstId":"r1","relType":"cites"}`, http.StatusCreated, `"id":"rel-1"`},
		{http.MethodGet, "/api/relationships?kind=artifact&id=a1", "", http.StatusOK, `[{"id":"rel-1"`},
		{http.MethodDelete, "/api/relationships/rel-1", "", http.StatusOK, `"ok":true`},
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
	if service.deleted != "rel-1" {
		t.Fatalf("deleted id = %q", service.deleted)
	}
}
