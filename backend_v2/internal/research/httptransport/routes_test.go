package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/research"
)

type researchServiceStub struct {
	created research.ResearchCreateInput
	patched research.ResearchPatch
	gotID   string
}

func (s *researchServiceStub) Create(_ context.Context, input research.ResearchCreateInput) (research.BrowserNodeResponse, error) {
	s.created = input
	return research.BrowserNodeResponse{ID: input.ID, Kind: "research", Title: input.Title}, nil
}

func (s *researchServiceStub) Get(_ context.Context, id string) (research.ResearchFolder, error) {
	s.gotID = id
	return research.ResearchFolder{NodeID: id, Title: "Momentum"}, nil
}

func (s *researchServiceStub) Update(_ context.Context, id string, patch research.ResearchPatch) (research.BrowserNodeResponse, error) {
	s.gotID, s.patched = id, patch
	return research.BrowserNodeResponse{ID: id, Kind: "research", Title: *patch.Title}, nil
}

func TestRegisterPreservesResearchRoutes(t *testing.T) {
	service := &researchServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	create := httptest.NewRecorder()
	mux.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/research", strings.NewReader(`{"id":"r1","title":"Momentum","hypothesis":"drift"}`)))
	if create.Code != http.StatusCreated || service.created.ID != "r1" || service.created.Hypothesis != "drift" {
		t.Fatalf("create status=%d input=%+v body=%s", create.Code, service.created, create.Body.String())
	}

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/research/r1", nil))
	if get.Code != http.StatusOK || service.gotID != "r1" {
		t.Fatalf("get status=%d id=%q body=%s", get.Code, service.gotID, get.Body.String())
	}

	patch := httptest.NewRecorder()
	mux.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, "/api/research/r1", strings.NewReader(`{"title":"Revised"}`)))
	if patch.Code != http.StatusOK || service.patched.Title == nil || *service.patched.Title != "Revised" {
		t.Fatalf("patch status=%d patch=%+v body=%s", patch.Code, service.patched, patch.Body.String())
	}
}
