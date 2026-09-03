package alert

import (
	"context"
	"sync"
	"testing"
	"time"

	"aladin/backend_v2/internal/market"
)

type fakeAlertRepo struct {
	mu       sync.Mutex
	active   []Alert
	fired    []firedRec
	fireErr  error
	disarmed map[string]bool
}

type firedRec struct {
	alertID string
	price   float64
}

func (r *fakeAlertRepo) Insert(context.Context, Alert) error { return nil }
func (r *fakeAlertRepo) ListByUser(context.Context, string) ([]Alert, error) {
	return nil, nil
}
func (r *fakeAlertRepo) ListActive(context.Context) ([]Alert, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Alert, len(r.active))
	copy(out, r.active)
	return out, nil
}
func (r *fakeAlertRepo) Fire(_ context.Context, alertID string, price float64, _ time.Time, _ Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fireErr != nil {
		return r.fireErr
	}
	if r.disarmed == nil {
		r.disarmed = map[string]bool{}
	}
	if r.disarmed[alertID] {
		return nil // idempotent: already fired (mirrors the DB armed=true guard)
	}
	r.disarmed[alertID] = true
	r.fired = append(r.fired, firedRec{alertID, price})
	return nil
}
func (r *fakeAlertRepo) Delete(context.Context, string, string) error            { return nil }
func (r *fakeAlertRepo) SetStatus(context.Context, string, string, string) error { return nil }
func (r *fakeAlertRepo) fires() []firedRec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]firedRec, len(r.fired))
	copy(out, r.fired)
	return out
}

type fakeDemand struct {
	mu         sync.Mutex
	subscribed map[string]bool
}

func (d *fakeDemand) Subscribe(_ context.Context, syms []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.subscribed == nil {
		d.subscribed = map[string]bool{}
	}
	for _, s := range syms {
		d.subscribed[s] = true
	}
	return nil
}
func (d *fakeDemand) Unsubscribe(_ context.Context, syms []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range syms {
		delete(d.subscribed, s)
	}
	return nil
}
func (d *fakeDemand) has(sym string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.subscribed[sym]
}

type fakeSnap map[string]float64

func (f fakeSnap) FetchSnapshot(_ context.Context, sym string) (market.Quote, bool, error) {
	if p, ok := f[sym]; ok {
		return market.Quote{Symbol: sym, Last: p}, true, nil
	}
	return market.Quote{}, false, nil
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

// TestEngineReconcileDrivesDemand — the engine subscribes alert symbols so they stream even
// with no UI demand.
func TestEngineReconcileDrivesDemand(t *testing.T) {
	repo := &fakeAlertRepo{active: []Alert{
		{ID: "a1", UserID: "u1", Symbol: "NVDA", Direction: AlertAbove, Threshold: 200, Armed: true, Status: "active"},
	}}
	demand := &fakeDemand{}
	eng := NewAlertEngine(repo, demand, fakeSnap{"NVDA": 190})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Start(ctx)
	waitFor(t, func() bool { return demand.has("NVDA") })
}

// TestEngineFiresOnLiveCross — a genuine upward cross through the threshold fires once.
func TestEngineFiresOnLiveCross(t *testing.T) {
	repo := &fakeAlertRepo{active: []Alert{
		{ID: "a1", UserID: "u1", Symbol: "NVDA", Direction: AlertAbove, Threshold: 200, Armed: true, Status: "active"},
	}}
	demand := &fakeDemand{}
	eng := NewAlertEngine(repo, demand, fakeSnap{"NVDA": 195})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Start(ctx)
	// Let the initial reconcile land (subscribes NVDA + seeds its slope from the 195 snapshot).
	waitFor(t, func() bool { return demand.has("NVDA") })
	time.Sleep(30 * time.Millisecond)

	// Feed a rising series through the observer (as the WS goroutine would).
	for _, p := range []float64{196, 197, 198, 199, 200.5, 201} {
		eng.OnTick("NVDA", p, time.Now())
		time.Sleep(2 * time.Millisecond)
	}
	waitFor(t, func() bool { return len(repo.fires()) == 1 })
	if got := repo.fires(); got[0].alertID != "a1" {
		t.Fatalf("fired %+v, want a1", got)
	}
}

// TestEngineBackstopFiresOffHoursCross — an alert armed at reconcile whose snapshot price is
// already past the threshold (it crossed while unwatched) fires via the backstop, no ticks.
func TestEngineBackstopFiresOffHoursCross(t *testing.T) {
	repo := &fakeAlertRepo{active: []Alert{
		{ID: "a1", UserID: "u1", Symbol: "NVDA", Direction: AlertAbove, Threshold: 200, Armed: true, Status: "active"},
	}}
	// Snapshot already above the threshold + still armed ⇒ crossed while we weren't watching.
	eng := NewAlertEngine(repo, &fakeDemand{}, fakeSnap{"NVDA": 205})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Start(ctx)
	waitFor(t, func() bool { return len(repo.fires()) == 1 })
}
