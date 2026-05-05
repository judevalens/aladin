package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/coder/websocket"
)

func (s *Server) registerRealtimeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events/ws", s.handleRealtimeWebSocket)
}

type realtimeClientMessage struct {
	Type          string                              `json:"type"`
	Subscriptions []coreservice.PublicSubscriptionKey `json:"subscriptions"`
}

type realtimeServerMessage struct {
	Type    string                `json:"type"`
	Event   *coreservice.AppEvent `json:"event,omitempty"`
	Message string                `json:"message,omitempty"`
}

func (s *Server) handleRealtimeWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	ctx := r.Context()
	readCtx, cancelRead := context.WithTimeout(ctx, 10*time.Second)
	_, data, err := conn.Read(readCtx)
	cancelRead()
	if err != nil {
		_ = writeRealtimeError(ctx, conn, "failed to read subscription message")
		return
	}

	var msg realtimeClientMessage
	if err := json.Unmarshal(data, &msg); err != nil || msg.Type != "subscribe" {
		_ = writeRealtimeError(ctx, conn, "expected subscribe message")
		return
	}

	keys, err := s.deps.RealtimeKeyResolver().ResolveSubscribeKeys(ctx, coreservice.SubscriptionOptions{
		Subscriptions: msg.Subscriptions,
	})
	if err != nil {
		_ = writeRealtimeError(ctx, conn, err.Error())
		return
	}

	events, unsubscribe, err := s.deps.Realtime().Subscribe(ctx, keys, r.Header.Get("Last-Event-ID"))
	if err != nil {
		_ = writeRealtimeError(ctx, conn, err.Error())
		return
	}
	defer unsubscribe()

	if err := writeRealtimeJSON(ctx, conn, realtimeServerMessage{Type: "subscribed"}); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := writeRealtimeJSON(ctx, conn, realtimeServerMessage{Type: "event", Event: &event}); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				return
			}
		}
	}
}

func writeRealtimeError(ctx context.Context, conn *websocket.Conn, message string) error {
	return writeRealtimeJSON(ctx, conn, realtimeServerMessage{Type: "error", Message: message})
}

func writeRealtimeJSON(ctx context.Context, conn *websocket.Conn, value any) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, bytes)
}
