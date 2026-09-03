package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/realtime"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type realtimeClientWireMessage struct {
	Type          string                           `json:"type"`
	Subscriptions []realtime.PublicSubscriptionKey `json:"subscriptions"`
}

type realtimeServerWireMessage struct {
	Type    string             `json:"type"`
	Event   *realtime.AppEvent `json:"event,omitempty"`
	Message string             `json:"message,omitempty"`
}

func TestRealtimeWebSocketResubscribeSwapsActiveSubscriptionSet(t *testing.T) {
	t.Parallel()

	resolver := realtime.NewSubscriptionKeyResolver(
		func(ctx context.Context) (string, error) {
			principal, err := auth.RequirePrincipal(ctx)
			return principal.UserID, err
		},
		func(ctx context.Context) error { return auth.RequireScope(ctx, auth.ScopeArtifactsRead) },
		func(message string) error { return apperror.BadRequest(message) },
	)
	realtimeService := realtime.NewService(resolver)
	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:      &fakeAuthService{},
		RealtimeSvc:  realtimeService,
		RealtimeKeys: resolver,
	})
	httpServer := httptest.NewServer(server.httpServer.Handler)
	t.Cleanup(httpServer.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	conn, _, err := websocket.Dial(ctx, websocketURL(httpServer.URL, "/api/events/ws"), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer desktop-valid"}},
	})
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") })

	writeRealtimeSubscribe(t, ctx, conn, []realtime.PublicSubscriptionKey{{
		Stream:       realtime.WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
	}})
	expectRealtimeServerMessage(t, ctx, conn, "subscribed", "")

	publishRealtimeEvent(t, realtimeService, "artifact", "artifact-1", "updated")
	expectRealtimeServerMessage(t, ctx, conn, "event", "artifact.updated")

	writeRealtimeSubscribe(t, ctx, conn, []realtime.PublicSubscriptionKey{{
		Stream:       realtime.WorkspaceStream,
		ResourceKind: "folder",
		ResourceID:   "folder-1",
	}})
	expectRealtimeServerMessage(t, ctx, conn, "subscribed", "")

	publishRealtimeEvent(t, realtimeService, "artifact", "artifact-1", "updated")
	publishRealtimeEvent(t, realtimeService, "folder", "folder-1", "updated")
	expectRealtimeServerMessage(t, ctx, conn, "event", "folder.updated")
}

func writeRealtimeSubscribe(t *testing.T, ctx context.Context, conn *websocket.Conn, subscriptions []realtime.PublicSubscriptionKey) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, realtimeClientWireMessage{
		Type:          "subscribe",
		Subscriptions: subscriptions,
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
}

func expectRealtimeServerMessage(t *testing.T, ctx context.Context, conn *websocket.Conn, messageType string, eventType string) realtimeServerWireMessage {
	t.Helper()
	var msg realtimeServerWireMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatalf("read realtime message: %v", err)
	}
	if msg.Type != messageType {
		t.Fatalf("message type = %q, want %q: %#v", msg.Type, messageType, msg)
	}
	if eventType != "" {
		if msg.Event == nil {
			t.Fatalf("event missing from message: %#v", msg)
		}
		if msg.Event.Type != eventType {
			t.Fatalf("event type = %q, want %q", msg.Event.Type, eventType)
		}
	}
	return msg
}

func publishRealtimeEvent(t *testing.T, realtimeService realtime.EventService, resourceKind string, resourceID string, operation string) {
	t.Helper()
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:    "user-1",
		ActorType: auth.ActorTypeUserSession,
		ActorID:   "user-1",
		Email:     "user@example.com",
	})
	if err := realtimeService.Publish(ctx, realtime.PublishTarget{
		Stream:       realtime.WorkspaceStream,
		ResourceKind: resourceKind,
		ResourceID:   resourceID,
		Operation:    operation,
	}, map[string]string{"id": resourceID}); err != nil {
		t.Fatalf("publish realtime event: %v", err)
	}
}

func websocketURL(baseURL string, path string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + path
}
