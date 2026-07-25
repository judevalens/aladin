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

// TestSessionDeliversTrades — REAL wire-shape trade frames reach onTrade. The frames
// below are verbatim from the live IEX feed (captured 2026-07-23): they carry BOTH
// "S" (symbol, string) and "s" (size, number). Go's case-insensitive JSON fallback
// used to fold the numeric "s" into the string S field, failing the unmarshal and
// silently dropping every trade — this test locks the fix. Never simplify these
// fixtures: the omitted fields WERE the bug.
func TestSessionDeliversTrades(t *testing.T) {
	srv := fakeAlpaca(t, []string{
		`[{"T":"success","msg":"connected"},{"T":"success","msg":"authenticated"}]`,
		`[{"T":"t","S":"MSFT","i":7927,"x":"V","p":379.88,"s":40,"c":["@"],"z":"C","t":"2026-07-23T14:55:08.275993595Z"}]`,
		`[{"T":"t","S":"NVDA","i":15527,"x":"V","p":207.37,"s":5,"c":["@","I"],"z":"C","t":"2026-07-23T14:55:08.79618075Z"}]`,
		`[{"T":"error","msg":"test over"}]`, // terminates the session cleanly
	})
	defer srv.Close()

	var got []Trade
	s := &Stream{url: wsURL(srv), symbols: map[string]bool{}, onTrade: func(tr Trade) { got = append(got, tr) }}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.session(ctx)

	if len(got) != 2 || got[0].Symbol != "MSFT" || got[0].Price != 379.88 || got[1].Symbol != "NVDA" || got[1].Price != 207.37 {
		t.Fatalf("trades = %+v, want MSFT @ 379.88 then NVDA @ 207.37", got)
	}
}

// TestStream_HealthAccounting covers the failure/liveness bookkeeping the staleness watchdog and
// health surface rely on: failures accumulate, a live frame resets them and stamps last-seen.
func TestStream_HealthAccounting(t *testing.T) {
	s := NewStream("ws://unused", "iex", "k", "sec", func(Trade) {})
	if st := s.Status(); st.Connected || st.ConsecutiveFailures != 0 || !st.LastMessageAt.IsZero() {
		t.Fatalf("fresh status = %+v, want disconnected/zero", st)
	}

	s.recordFailure()
	s.recordFailure()
	s.recordFailure()
	if got := s.Status().ConsecutiveFailures; got != 3 {
		t.Fatalf("after 3 failures = %d, want 3", got)
	}

	s.markAlive()
	st := s.Status()
	if st.ConsecutiveFailures != 0 {
		t.Fatalf("markAlive must reset failures, got %d", st.ConsecutiveFailures)
	}
	if st.LastMessageAt.IsZero() {
		t.Fatalf("markAlive must stamp LastMessageAt")
	}
}
