package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/graphpane"
)

type graphPaneServiceStub struct{ artifactID string }

func (s *graphPaneServiceStub) ForArtifact(_ context.Context, artifactID string) (*graphpane.GraphPane, error) {
	s.artifactID = artifactID
	return &graphpane.GraphPane{Entities: []graphpane.GraphEntity{}, LinkedArtifacts: []graphpane.GraphLinkedArtifact{}}, nil
}

func TestRegisterPreservesGraphPaneRoute(t *testing.T) {
	service := &graphPaneServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/graph-pane?artifact=a1", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"entities":[]`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.artifactID != "a1" {
		t.Fatalf("artifact id = %q", service.artifactID)
	}

	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/graph-pane", nil))
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing artifact status = %d", missing.Code)
	}
}
