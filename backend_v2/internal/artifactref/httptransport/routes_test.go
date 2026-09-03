package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/artifactref"
)

type refServiceStub struct {
	query      string
	artifactID string
	refs       []artifactref.ArtifactRef
}

func (s *refServiceStub) Search(_ context.Context, _ string, query string, _ int) ([]artifactref.RefHit, error) {
	s.query = query
	return []artifactref.RefHit{{Kind: artifactref.RefKindPage, ID: "p1", Label: "Plan"}}, nil
}

func (s *refServiceStub) SyncRefs(_ context.Context, artifactID string, refs []artifactref.ArtifactRef) error {
	s.artifactID, s.refs = artifactID, refs
	return nil
}

func (s *refServiceStub) ListForArtifact(_ context.Context, artifactID string) ([]artifactref.AttachedRef, error) {
	s.artifactID = artifactID
	return []artifactref.AttachedRef{{Kind: artifactref.RefKindPage, TargetID: "p1", Label: "Plan"}}, nil
}

func TestRegisterPreservesArtifactReferenceRoutes(t *testing.T) {
	service := &refServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	search := httptest.NewRecorder()
	mux.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/api/refs/search?q=plan", nil))
	if search.Code != http.StatusOK || service.query != "plan" {
		t.Fatalf("search status=%d query=%q body=%s", search.Code, service.query, search.Body.String())
	}

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/artifacts/a1/refs", nil))
	if list.Code != http.StatusOK || service.artifactID != "a1" {
		t.Fatalf("list status=%d artifact=%q body=%s", list.Code, service.artifactID, list.Body.String())
	}

	sync := httptest.NewRecorder()
	mux.ServeHTTP(sync, httptest.NewRequest(http.MethodPut, "/api/artifacts/a1/refs", strings.NewReader(`{"refs":[{"kind":"page","targetId":"p1","blockId":"b1","surface":"Plan"}]}`)))
	if sync.Code != http.StatusNoContent || len(service.refs) != 1 || service.refs[0].TargetID != "p1" {
		t.Fatalf("sync status=%d refs=%+v body=%s", sync.Code, service.refs, sync.Body.String())
	}
}
