package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/record"
)

type fakeService struct {
	createdType    string
	createdContent string
}

func (*fakeService) List(context.Context) ([]record.RecordResponse, error) {
	return []record.RecordResponse{{ID: "record-1", Type: "note", Label: "One"}}, nil
}
func (*fakeService) Children(context.Context, string, int, int) (map[string]any, error) {
	return map[string]any{"items": []record.RecordResponse{}, "total": 0}, nil
}
func (f *fakeService) Create(_ context.Context, _, kind, _, content, _, _ string) error {
	f.createdType = kind
	f.createdContent = content
	return nil
}
func (*fakeService) Delete(context.Context, string) error        { return nil }
func (*fakeService) Retry(context.Context, string) (bool, error) { return true, nil }
func (*fakeService) Similar(context.Context, string, int) ([]record.SimilarRecord, error) {
	return nil, nil
}

func TestRecordRoutesPreserveListAndCreateContracts(t *testing.T) {
	service := &fakeService{}
	mux := http.NewServeMux()
	Register(mux, service)

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/records/", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	var records []record.RecordResponse
	if err := json.Unmarshal(list.Body.Bytes(), &records); err != nil || len(records) != 1 || records[0].ID != "record-1" {
		t.Fatalf("list body = %s, err = %v", list.Body.String(), err)
	}

	create := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/records/", strings.NewReader(`{"type":"note","content":"hello"}`))
	mux.ServeHTTP(create, req)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", create.Code, create.Body.String())
	}
	if service.createdType != "note" || service.createdContent != "hello" {
		t.Fatalf("create forwarded type=%q content=%q", service.createdType, service.createdContent)
	}
}
