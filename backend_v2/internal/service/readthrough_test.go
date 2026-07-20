package service

import (
	"context"
	"testing"
	"time"
)

// ── bars read-through ──

type fakeBarRepo struct{ bars map[string][]Bar }

func (f *fakeBarRepo) ListBars(_ context.Context, symbol, tf string, limit int) ([]Bar, error) {
	b := f.bars[symbol+"|"+tf]
	if len(b) > limit {
		b = b[len(b)-limit:]
	}
	return append([]Bar{}, b...), nil
}
func (f *fakeBarRepo) UpsertBars(_ context.Context, rows []BarUpsert) (int, error) {
	for _, r := range rows {
		key := r.Symbol + "|" + r.Timeframe
		f.bars[key] = append(f.bars[key], r.Bar)
	}
	return len(rows), nil
}

type fakeBarSource struct {
	bars  []Bar
	calls int
}

func (f *fakeBarSource) FetchBars(context.Context, string, string, string, string) ([]Bar, error) {
	f.calls++
	return f.bars, nil
}

func TestBarsReadThroughFillsColdThenThrottles(t *testing.T) {
	repo := &fakeBarRepo{bars: map[string][]Bar{}}
	src := &fakeBarSource{bars: []Bar{
		{Time: time.Now().AddDate(0, 0, -2), Close: 184},
		{Time: time.Now().AddDate(0, 0, -1), Close: 189}, // newest is ~1d old → stale after first fill
	}}
	svc := NewBarService(repo).WithSource(src)
	ctx := context.Background()

	// Cold cache → one vendor fetch → returns the filled bars.
	got, err := svc.Get(ctx, "AAPL", "1Day", 10)
	if err != nil || len(got) != 2 || src.calls != 1 {
		t.Fatalf("cold fill: got %d bars, %d calls, err=%v", len(got), src.calls, err)
	}
	// Stale but throttled → no second vendor call within the window.
	if _, err := svc.Get(ctx, "AAPL", "1Day", 10); err != nil || src.calls != 1 {
		t.Fatalf("throttle: calls=%d err=%v (want 1)", src.calls, err)
	}
}

func TestBarsNoSourceReturnsLocalOnly(t *testing.T) {
	repo := &fakeBarRepo{bars: map[string][]Bar{"AAPL|1Day": {{Time: time.Now(), Close: 227}}}}
	got, err := NewBarService(repo).Get(context.Background(), "AAPL", "1Day", 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("local-only: got %d err=%v", len(got), err)
	}
}

// ── instruments on-demand resolve ──

type fakeInstRepo struct {
	ids     map[string]string
	upserts int
}

func (f *fakeInstRepo) SearchInstruments(context.Context, string, int) ([]InstrumentHit, error) {
	return nil, nil
}
func (f *fakeInstRepo) UpsertInstruments(_ context.Context, rows []InstrumentUpsert) (int, error) {
	for _, r := range rows {
		f.ids[r.Symbol] = "id-" + r.Symbol
	}
	f.upserts++
	return len(rows), nil
}
func (f *fakeInstRepo) ResolveInstrumentID(_ context.Context, symbol string) (string, bool, error) {
	id, ok := f.ids[symbol]
	return id, ok, nil
}

type fakeLookup struct{ calls int }

func (f *fakeLookup) FetchInstrument(_ context.Context, symbol string) (InstrumentUpsert, bool, error) {
	f.calls++
	return InstrumentUpsert{Symbol: symbol, IsActive: true}, true, nil
}

func TestInstrumentResolveReadThroughOnMiss(t *testing.T) {
	repo := &fakeInstRepo{ids: map[string]string{}}
	lookup := &fakeLookup{}
	svc := NewInstrumentService(repo).WithAssetLookup(lookup)
	ctx := context.Background()

	// Miss → vendor lookup → upsert → resolves.
	id, ok, err := svc.ResolveInstrumentID(ctx, "ZZZ")
	if err != nil || !ok || id != "id-ZZZ" || lookup.calls != 1 || repo.upserts != 1 {
		t.Fatalf("read-through: id=%q ok=%v calls=%d upserts=%d err=%v", id, ok, lookup.calls, repo.upserts, err)
	}
	// Now cached in the repo → no second vendor lookup.
	if _, _, err := svc.ResolveInstrumentID(ctx, "ZZZ"); err != nil || lookup.calls != 1 {
		t.Fatalf("cached: calls=%d err=%v (want 1)", lookup.calls, err)
	}
}
