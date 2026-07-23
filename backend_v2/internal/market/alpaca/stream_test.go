package alpaca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeAlpaca upgrades to a WS and plays scripted frames (each one raw JSON), then
// keeps the connection open until the client goes away or the test ends.
func fakeAlpaca(t *testing.T, frames []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		ctx := r.Context()
		for _, f := range frames {
			if err := conn.Write(ctx, websocket.MessageText, []byte(f)); err != nil {
				return
			}
		}
		// Drain client commands (auth/subscribe) until the test finishes.
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}))
}

func wsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

// TestSessionDiesOnUpstreamError — an Alpaca `error` control frame (auth failure,
// "connection limit exceeded") must TERMINATE the session so Run can reconnect
// with backoff. Ignoring it leaves a zombie connection that itself counts
// against Alpaca's connection limit, deadlocking the slot permanently.
func TestSessionDiesOnUpstreamError(t *testing.T) {
	srv := fakeAlpaca(t, []string{
		`[{"T":"success","msg":"connected"}]`,
		`[{"T":"error","msg":"connection limit exceeded"}]`,
	})
	defer srv.Close()

	s := &Stream{url: wsURL(srv), symbols: map[string]bool{}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s.session(ctx)
	if err == nil || !strings.Contains(err.Error(), "connection limit exceeded") {
		t.Fatalf("session error = %v, want upstream 'connection limit exceeded'", err)
	}
	if ctx.Err() != nil {
		t.Fatal("session only ended because the test timed out — the error frame did not terminate it")
	}
}

// TestSessionDeliversTrades — trade frames reach onTrade with symbol + price.
func TestSessionDeliversTrades(t *testing.T) {
	srv := fakeAlpaca(t, []string{
		`[{"T":"success","msg":"connected"},{"T":"success","msg":"authenticated"}]`,
		`[{"T":"t","S":"NVDA","p":208.65,"t":"2026-07-23T14:30:00Z"}]`,
		`[{"T":"error","msg":"test over"}]`, // terminates the session cleanly
	})
	defer srv.Close()

	var got []Trade
	s := &Stream{url: wsURL(srv), symbols: map[string]bool{}, onTrade: func(tr Trade) { got = append(got, tr) }}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.session(ctx)

	if len(got) != 1 || got[0].Symbol != "NVDA" || got[0].Price != 208.65 {
		t.Fatalf("trades = %+v, want one NVDA @ 208.65", got)
	}
}
