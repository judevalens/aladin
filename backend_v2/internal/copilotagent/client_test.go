package copilotagent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestStartTurn_HeartbeatsKeepQuietTurnAlive(t *testing.T) {
	old := streamIdleTimeout
	streamIdleTimeout = 150 * time.Millisecond
	t.Cleanup(func() { streamIdleTimeout = old })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := w.(http.Flusher)
		fmt.Fprintln(w, `{"type":"tool_start","name":"build_app"}`)
		f.Flush()
		for i := 0; i < 12; i++ {
			select {
			case <-time.After(30 * time.Millisecond):
				fmt.Fprintln(w, `{"type":"heartbeat"}`)
				f.Flush()
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprintln(w, `{"type":"done"}`)
		f.Flush()
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	events, err := New(srv.URL, "").StartTurn(ctx, TurnRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for event := range events {
		types = append(types, event.Type)
	}
	if got := strings.Join(types, ","); got != "tool_start,done" {
		t.Fatalf("quiet turn events = %s; want tool_start,done with no heartbeat UI events", got)
	}
}

func TestStartTurn_DoneDoesNotWaitForHTTPToClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"type":"done"}`)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	events, err := New(srv.URL, "").StartTurn(ctx, TurnRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if event := <-events; event.Type != "done" {
		t.Fatalf("first event = %+v, want done", event)
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("event after done")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("terminal done waited for HTTP EOF")
	}
}

func TestStartTurn_HeartbeatsDoNotOverrideTurnDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		for {
			select {
			case <-ticker.C:
				fmt.Fprintln(w, `{"type":"heartbeat"}`)
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	events, err := New(srv.URL, "").StartTurn(ctx, TurnRequest{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("transport heartbeat was exposed as a model event")
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeats defeated turn deadline")
	}
}

func TestStartTurn_BrokenStreamsNeverInventDone(t *testing.T) {
	for _, line := range []string{`{"type":"thinking"}`, strings.Repeat("x", 1024*1024+1)} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, line)
		}))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		events, err := New(srv.URL, "").StartTurn(ctx, TurnRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for event := range events {
			if event.Type == "done" {
				t.Error("broken stream reported clean completion")
			}
		}
		cancel()
		srv.Close()
	}
}
