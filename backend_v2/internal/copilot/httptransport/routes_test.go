package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"aladin/backend_v2/internal/copilot"
)

type fakeCopilotService struct{}

func (*fakeCopilotService) SendMessage(context.Context, copilot.CopilotSendInput) (copilot.CopilotSendResult, error) {
	return copilot.CopilotSendResult{}, nil
}
func (*fakeCopilotService) ListThreads(context.Context, string) ([]copilot.CopilotThread, error) {
	return nil, nil
}
func (*fakeCopilotService) GetThread(context.Context, string, string) (copilot.CopilotThreadDetail, error) {
	return copilot.CopilotThreadDetail{}, nil
}
func (*fakeCopilotService) RenameThread(context.Context, string, string, string) (copilot.CopilotThread, error) {
	return copilot.CopilotThread{}, nil
}
func (*fakeCopilotService) ArchiveThread(context.Context, string, string) error { return nil }
func (*fakeCopilotService) SetThreadPinned(context.Context, string, string, bool) (copilot.CopilotThread, error) {
	return copilot.CopilotThread{}, nil
}
func (*fakeCopilotService) Cancel(context.Context, string, string) error        { return nil }
func (*fakeCopilotService) ApproveAction(context.Context, string, string) error { return nil }
func (*fakeCopilotService) RejectAction(context.Context, string, string) error  { return nil }
func (*fakeCopilotService) Configured() bool                                    { return false }
func (*fakeCopilotService) Status(context.Context) copilot.CopilotStatusReport {
	return copilot.CopilotStatusReport{Configured: false}
}

func TestStatusRoutePreservesCopilotContract(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &fakeCopilotService{})
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/copilot/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Body.String(); got != "{\"configured\":false,\"sidecar\":false,\"mcp\":false}\n" {
		t.Fatalf("body = %q", got)
	}
}
