package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSystemService struct{}

func (*fakeSystemService) Ready(context.Context) error { return nil }
func (*fakeSystemService) WorkerStatus(context.Context) (map[string]any, error) {
	return map[string]any{"workerUp": true}, nil
}
func (*fakeSystemService) PipelineStats(context.Context) (map[string]any, error) {
	return map[string]any{"records": map[string]any{"inFlight": 2}}, nil
}

func TestSystemRoutesPreserveStatusEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &fakeSystemService{})

	for _, tc := range []struct {
		path string
		key  string
	}{
		{path: "/api/worker/status", key: "workerUp"},
		{path: "/api/pipeline/stats", key: "records"},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", tc.path, recorder.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s body: %v", tc.path, err)
		}
		if _, ok := body[tc.key]; !ok {
			t.Fatalf("%s body missing %q: %s", tc.path, tc.key, recorder.Body.String())
		}
	}
}
