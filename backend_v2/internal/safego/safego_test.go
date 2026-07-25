package safego

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGo_RecoversPanic proves a panicking fire-and-forget goroutine is contained
// (the test process survives) and runs to the panic point exactly once.
func TestGo_RecoversPanic(t *testing.T) {
	var ran atomic.Int32
	done := make(chan struct{})
	Go("test.go", func() {
		ran.Add(1)
		close(done)
		panic("boom") // must be recovered, not crash the test binary
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine never ran")
	}
	// Give the deferred recover a moment; if it didn't recover, the process is already dead.
	time.Sleep(20 * time.Millisecond)
	if got := ran.Load(); got != 1 {
		t.Fatalf("ran %d times, want 1 (no restart for Go)", got)
	}
}

// TestLoop_RestartsAfterPanic proves a supervised loop that panics on its first
// entry is restarted and then runs healthily, and that ctx cancel stops it.
func TestLoop_RestartsAfterPanic(t *testing.T) {
	// Shorten the restart backoff for the test via a fast ctx timeout envelope.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	healthy := make(chan struct{})
	var once sync.Once
	Loop(ctx, "test.loop", func(ctx context.Context) {
		n := calls.Add(1)
		if n == 1 {
			panic("boom on first entry")
		}
		// Second entry: signal healthy, then block until ctx is cancelled.
		once.Do(func() { close(healthy) })
		<-ctx.Done()
	})

	select {
	case <-healthy:
	case <-time.After(3 * time.Second):
		t.Fatalf("loop did not restart after panic (calls=%d)", calls.Load())
	}
	if got := calls.Load(); got < 2 {
		t.Fatalf("calls=%d, want >=2 (restarted after panic)", got)
	}

	// Cancelling ctx must stop the loop (no further restarts).
	cancel()
	time.Sleep(restartBackoff + 200*time.Millisecond)
	stable := calls.Load()
	time.Sleep(restartBackoff + 200*time.Millisecond)
	if calls.Load() != stable {
		t.Fatalf("loop kept restarting after ctx cancel: %d -> %d", stable, calls.Load())
	}
}
