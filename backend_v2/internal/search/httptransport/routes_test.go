package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/search"
	coreservice "aladin/backend_v2/internal/service"
)

type searchServiceStub struct {
	userID string
	query  string
	limit  int
}

func (s *searchServiceStub) Search(_ context.Context, userID, query string, limit int) (search.SearchResponse, error) {
	s.userID, s.query, s.limit = userID, query, limit
	return search.SearchResponse{Sections: []search.SearchSection{{
		Type: "entity", Label: "Entities", Hits: []search.SearchHit{{Kind: "ticker", ID: "AAPL", Title: "Apple"}},
	}}}, nil
}

func TestRegisterPreservesGlobalSearchContract(t *testing.T) {
	service := &searchServiceStub{}
	mux := http.NewServeMux()
	Register(mux, service)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=apple&limit=4", nil)
	req = req.WithContext(coreservice.WithPrincipal(req.Context(), coreservice.Principal{UserID: "u1"}))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if service.userID != "u1" || service.query != "apple" || service.limit != 4 {
		t.Fatalf("service args = (%q, %q, %d)", service.userID, service.query, service.limit)
	}
	if !strings.Contains(response.Body.String(), `"sections":[{"type":"entity"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}
