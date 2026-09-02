package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestRealtimeWebSocketResubscribeSwapsActiveSubscriptionSet(t *testing.T) {
	t.Parallel()

	resolver := artifactservice.NewSubscriptionKeyResolver()
	realtime := artifactservice.NewInMemoryRealtimeEventService(resolver)
	server := NewWithDependencies(":0", testDependencies{
		AuthSvc:      &fakeAuthService{},
		RealtimeSvc:  realtime,
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

	writeRealtimeSubscribe(t, ctx, conn, []artifactservice.PublicSubscriptionKey{{
		Stream:       artifactservice.WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
	}})
	expectRealtimeServerMessage(t, ctx, conn, "subscribed", "")

	publishRealtimeEvent(t, realtime, "artifact", "artifact-1", "updated")
	expectRealtimeServerMessage(t, ctx, conn, "event", "artifact.updated")

	writeRealtimeSubscribe(t, ctx, conn, []artifactservice.PublicSubscriptionKey{{
		Stream:       artifactservice.WorkspaceStream,
		ResourceKind: "folder",
		ResourceID:   "folder-1",
	}})
	expectRealtimeServerMessage(t, ctx, conn, "subscribed", "")

	publishRealtimeEvent(t, realtime, "artifact", "artifact-1", "updated")
	publishRealtimeEvent(t, realtime, "folder", "folder-1", "updated")
	expectRealtimeServerMessage(t, ctx, conn, "event", "folder.updated")
}

func writeRealtimeSubscribe(t *testing.T, ctx context.Context, conn *websocket.Conn, subscriptions []artifactservice.PublicSubscriptionKey) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, realtimeClientMessage{
		Type:          "subscribe",
		Subscriptions: subscriptions,
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
}

func expectRealtimeServerMessage(t *testing.T, ctx context.Context, conn *websocket.Conn, messageType string, eventType string) realtimeServerMessage {
	t.Helper()
	var msg realtimeServerMessage
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

func publishRealtimeEvent(t *testing.T, realtime artifactservice.RealtimeEventService, resourceKind string, resourceID string, operation string) {
	t.Helper()
	ctx := artifactservice.WithPrincipal(context.Background(), artifactservice.Principal{
		UserID:    "user-1",
		ActorType: artifactservice.ActorTypeUserSession,
		ActorID:   "user-1",
		Email:     "user@example.com",
	})
	if err := realtime.Publish(ctx, artifactservice.PublishTarget{
		Stream:       artifactservice.WorkspaceStream,
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
