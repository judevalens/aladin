package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/graph"
)

type fakeGraphReader struct {
	nb      *graph.Neighborhood
	gotID   string
	gotLim  int
	callErr error
}

func (f *fakeGraphReader) Neighbors(_ context.Context, id string, limit int) (*graph.Neighborhood, error) {
	f.gotID = id
	f.gotLim = limit
	if f.callErr != nil {
		return nil, f.callErr
	}
	return f.nb, nil
}

func TestGraphNeighbors_NotConfiguredReturns503(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", app.StaticDependencies{}) // GraphReaderSvc nil
	req := httptest.NewRequest(http.MethodGet, "/api/graph/entity/ent-1/neighbors", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
}

func TestGraphNeighbors_ReturnsNeighborhood(t *testing.T) {
	t.Parallel()

	reader := &fakeGraphReader{nb: &graph.Neighborhood{
		Entity:  graph.NeighborEntity{ID: "ent-1", Name: "Acme", Kind: "org"},
		Related: []graph.NeighborEntity{{ID: "ent-2", Name: "Globex", Kind: "org", Weight: 3}},
	}}
	server := NewWithDependencies(":0", app.StaticDependencies{GraphReaderSvc: reader})

	req := httptest.NewRequest(http.MethodGet, "/api/graph/entity/ent-1/neighbors?limit=5", nil)
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if reader.gotID != "ent-1" {
		t.Fatalf("entity id = %q, want ent-1", reader.gotID)
	}
	if reader.gotLim != 5 {
		t.Fatalf("limit = %d, want 5", reader.gotLim)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Globex") || !strings.Contains(body, "Acme") {
		t.Fatalf("body missing neighbourhood payload: %s", body)
	}
}
