package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"aladin/backend_v2/internal/realtime"
	"aladin/backend_v2/internal/safego"

	"github.com/coder/websocket"
)

type routes struct {
	events   realtime.EventService
	resolver realtime.KeyResolver
}

func Register(mux *http.ServeMux, events realtime.EventService, resolver realtime.KeyResolver) {
	r := routes{events: events, resolver: resolver}
	mux.HandleFunc("GET /api/events/ws", r.handleRealtimeWebSocket)
}

type realtimeClientMessage struct {
	Type          string                           `json:"type"`
	Subscriptions []realtime.PublicSubscriptionKey `json:"subscriptions"`
}

type realtimeServerMessage struct {
	Type    string             `json:"type"`
	Event   *realtime.AppEvent `json:"event,omitempty"`
	Message string             `json:"message,omitempty"`
}

func (h routes) handleRealtimeWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	ctx := r.Context()
	msg, err := readRealtimeClientMessage(ctx, conn, 10*time.Second)
	if err != nil || msg.Type != "subscribe" {
		_ = writeRealtimeError(ctx, conn, "expected subscribe message")
		return
	}

	keys, err := h.resolver.ResolveSubscribeKeys(ctx, realtime.SubscriptionOptions{
		Subscriptions: msg.Subscriptions,
	})
	if err != nil {
		_ = writeRealtimeError(ctx, conn, err.Error())
		return
	}

	lastEventID := r.Header.Get("Last-Event-ID")
	events, unsubscribe, err := h.events.Subscribe(ctx, keys, lastEventID)
	if err != nil {
		_ = writeRealtimeError(ctx, conn, err.Error())
		return
	}
	defer unsubscribe()

	clientMessages := make(chan realtimeClientMessage)
	clientErrors := make(chan error, 1)
	// Panic-contained: a malformed frame from a WS client must not crash the api process.
	safego.Go("api.realtime.read", func() { readRealtimeClientMessages(ctx, conn, clientMessages, clientErrors) })

	if err := writeRealtimeJSON(ctx, conn, realtimeServerMessage{Type: "subscribed"}); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-clientErrors:
			if errors.Is(err, context.Canceled) {
				return
			}
			return
		case msg, ok := <-clientMessages:
			if !ok {
				return
			}
			if msg.Type != "subscribe" {
				_ = writeRealtimeError(ctx, conn, "expected subscribe message")
				return
			}
			nextKeys, err := h.resolver.ResolveSubscribeKeys(ctx, realtime.SubscriptionOptions{
				Subscriptions: msg.Subscriptions,
			})
			if err != nil {
				_ = writeRealtimeError(ctx, conn, err.Error())
				return
			}
			nextEvents, nextUnsubscribe, err := h.events.Subscribe(ctx, nextKeys, lastEventID)
			if err != nil {
				_ = writeRealtimeError(ctx, conn, err.Error())
				return
			}
			unsubscribe()
			events = nextEvents
			unsubscribe = nextUnsubscribe
			if err := writeRealtimeJSON(ctx, conn, realtimeServerMessage{Type: "subscribed"}); err != nil {
				return
			}
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
			lastEventID = event.EventID
		}
	}
}

func readRealtimeClientMessages(ctx context.Context, conn *websocket.Conn, messages chan<- realtimeClientMessage, errors chan<- error) {
	defer close(messages)
	for {
		msg, err := readRealtimeClientMessage(ctx, conn, 0)
		if err != nil {
			errors <- err
			return
		}
		select {
		case messages <- msg:
		case <-ctx.Done():
			errors <- ctx.Err()
			return
		}
	}
}

func readRealtimeClientMessage(ctx context.Context, conn *websocket.Conn, timeout time.Duration) (realtimeClientMessage, error) {
	readCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		readCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	_, data, err := conn.Read(readCtx)
	if err != nil {
		return realtimeClientMessage{}, err
	}
	var msg realtimeClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return realtimeClientMessage{}, err
	}
	return msg, nil
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
