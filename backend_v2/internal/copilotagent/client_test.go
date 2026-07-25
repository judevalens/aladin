package copilotagent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStartTurn_IdleTimeoutClosesStream proves the idle watchdog aborts a hung mid-turn stream:
// the sidecar sends one event then stops (never sends `done`, never closes), and the client must
// close the events channel (not block until the 15-min turn ctx). If the watchdog didn't fire,
// the range loop below would block until the test times out.
func TestStartTurn_IdleTimeoutClosesStream(t *testing.T) {
	old := streamIdleTimeout
	streamIdleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { streamIdleTimeout = old })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"type":"text","text":"hi"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done() // hang until the client's idle-cancel aborts the request
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	events, err := c.StartTurn(context.Background(), TurnRequest{ThreadID: "t1"})
	if err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	sawDone, count := false, 0
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok { // channel closed → watchdog fired
				if sawDone {
					t.Fatal("stream hung but reported done")
				}
				if count == 0 {
					t.Fatal("expected the one pre-hang event")
				}
				return
			}
			count++
			if ev.Type == "done" {
				sawDone = true
			}
		case <-deadline:
			t.Fatal("events channel never closed — idle watchdog did not fire")
		}
	}
}
