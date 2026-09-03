package market

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeStream records upstream subscribe/unsubscribe calls to prove the hub's dedup.
type fakeStream struct {
	mu       sync.Mutex
	subbed   []string
	unsubbed []string
}

func (f *fakeStream) Subscribe(_ context.Context, symbols ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subbed = append(f.subbed, symbols...)
}
func (f *fakeStream) Unsubscribe(_ context.Context, symbols ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsubbed = append(f.unsubbed, symbols...)
}
func (f *fakeStream) Run(context.Context) {}

type lifecycleStream struct {
	started chan struct{}
	stopped chan struct{}
}

func (f *lifecycleStream) Subscribe(context.Context, ...string)   {}
func (f *lifecycleStream) Unsubscribe(context.Context, ...string) {}
func (f *lifecycleStream) Run(ctx context.Context) {
	f.started <- struct{}{}
	<-ctx.Done()
	f.stopped <- struct{}{}
}

type stubResolver struct{}

func (stubResolver) ResolveInstrumentID(_ context.Context, symbol string) (string, bool, error) {
	return "id-" + symbol, true, nil
}

func TestMarketDataHubRefcountDedup(t *testing.T) {
	fs := &fakeStream{}
	h := newHub(nil, stubResolver{})
	h.stream = fs
	ctx := context.Background()

	// Two clients want NVDA, one wants AAPL → exactly one upstream sub per symbol.
	_ = h.Subscribe(ctx, []string{"NVDA"})
	_ = h.Subscribe(ctx, []string{"NVDA", "AAPL"})
	if got := fs.subbed; len(got) != 2 {
		t.Fatalf("upstream subs = %v, want one each for NVDA + AAPL", got)
	}

	// First NVDA release: still one client → no upstream unsub.
	_ = h.Unsubscribe(ctx, []string{"NVDA"})
	if len(fs.unsubbed) != 0 {
		t.Fatalf("unsub too early: %v", fs.unsubbed)
	}
	// Last NVDA release → one upstream unsub.
	_ = h.Unsubscribe(ctx, []string{"NVDA"})
	if len(fs.unsubbed) != 1 || fs.unsubbed[0] != "NVDA" {
		t.Fatalf("expected one NVDA unsub, got %v", fs.unsubbed)
	}
}

func TestMarketDataHubConnectsOnlyOnDemandAndReleasesWhenIdle(t *testing.T) {
	stream := &lifecycleStream{
		started: make(chan struct{}, 1),
		stopped: make(chan struct{}, 1),
	}
	h := newHub(nil, stubResolver{})
	h.stream = stream
	h.idleTimeout = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)
	select {
	case <-stream.started:
		t.Fatal("idle hub opened the upstream stream")
	case <-time.After(20 * time.Millisecond):
	}

	_ = h.Subscribe(ctx, []string{"AAPL"})
	select {
	case <-stream.started:
	case <-time.After(time.Second):
		t.Fatal("first subscription did not start the upstream stream")
	}

	_ = h.Unsubscribe(ctx, []string{"AAPL"})
	select {
	case <-stream.stopped:
	case <-time.After(time.Second):
		t.Fatal("idle hub did not release the upstream stream")
	}
}

type capturingQuotes struct{ ch chan Quote }

func (c capturingQuotes) Publish(_ context.Context, q Quote) error { c.ch <- q; return nil }

type fakeSnapshots struct{}

func (fakeSnapshots) FetchSnapshot(_ context.Context, _ string) (Quote, bool, error) {
	return Quote{Last: 100, PrevClose: 95, Change: 5, ChangePct: 5.26}, true, nil
}

func TestHubSeedsSnapshotOnSubscribe(t *testing.T) {
	quotes := capturingQuotes{ch: make(chan Quote, 1)}
	h := newHub(quotes, stubResolver{})
	h.snapshots = fakeSnapshots{}
	h.stream = &fakeStream{}

	_ = h.Subscribe(context.Background(), []string{"AAPL"})

	select {
	case q := <-quotes.ch:
		if q.Symbol != "AAPL" || q.InstrumentID != "id-AAPL" || q.Last != 100 || q.PrevClose != 95 {
			t.Fatalf("seed quote wrong: %+v", q)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no seed quote published on subscribe")
	}
}
