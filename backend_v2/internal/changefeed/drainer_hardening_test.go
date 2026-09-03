package changefeed

import (
	"context"
	"errors"
	"testing"

	. "aladin/backend_v2/internal/service"
)

// flakyRealtime fails Publish for the first failUntil calls, then succeeds. Records how many
// publishes were attempted so a test can assert the drain stopped at the failing event.
type flakyRealtime struct {
	attempts int
	failAt   int // the 1-based attempt number that fails (0 = never)
}

func (f *flakyRealtime) Publish(context.Context, PublishTarget, any) error {
	f.attempts++
	if f.attempts == f.failAt {
		return errors.New("realtime publish failed")
	}
	return nil
}
func (f *flakyRealtime) Subscribe(context.Context, []SubscriptionKey, string) (<-chan AppEvent, func(), error) {
	return nil, func() {}, nil
}

// horizonReader is a drain reader whose Horizon errors errsBeforeOK times before succeeding, and
// whose DrainSince returns a fixed window. Used to exercise initCursor's retry.
type horizonReader struct {
	events       []DrainedEvent
	next         uint64
	horizon      uint64
	horizonFails int // remaining Horizon calls that error
}

func (h *horizonReader) DrainSince(context.Context, uint64) ([]DrainedEvent, uint64, error) {
	return h.events, h.next, nil
}
func (h *horizonReader) Horizon(context.Context) (uint64, error) {
	if h.horizonFails > 0 {
		h.horizonFails--
		return 0, errors.New("db not ready")
	}
	return h.horizon, nil
}

// TestDrainOnce_PublishFailureRetriesFromFailedEvent proves a publish failure does NOT burn the
// window off the live channel: the drain returns the failed event's xid so [failed, …) retries,
// and the events before it are not re-published.
func TestDrainOnce_PublishFailureRetriesFromFailedEvent(t *testing.T) {
	frame := Frame{Entities: []FrameEntity{{EntityKind: "folder", EntityID: "f", Seq: 1, Op: OpUpsert}}}
	reader := &horizonReader{
		events: []DrainedEvent{
			{Xid: 10, UserID: "u1", Frame: frame},
			{Xid: 11, UserID: "u1", Frame: frame}, // publish of this one fails
			{Xid: 12, UserID: "u1", Frame: frame},
		},
		next: 100,
	}
	rt := &flakyRealtime{failAt: 2} // second publish fails
	d := NewDrainer(reader, rt, 0)

	next, err := d.drainOnce(context.Background(), 0)
	if err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if next != 11 {
		t.Fatalf("cursor = %d, want 11 (retry from the failed event, not horizon)", next)
	}
	if rt.attempts != 2 {
		t.Fatalf("publish attempts = %d, want 2 (stopped at the failure, didn't publish xid 12)", rt.attempts)
	}
}

// TestDrainOnce_AllSucceedAdvancesToNext proves the happy path advances to the reader's next cursor.
func TestDrainOnce_AllSucceedAdvancesToNext(t *testing.T) {
	reader := &horizonReader{
		events: []DrainedEvent{{Xid: 5, UserID: "u1", Frame: Frame{}}},
		next:   77,
	}
	rt := &flakyRealtime{failAt: 0}
	d := NewDrainer(reader, rt, 0)
	next, err := d.drainOnce(context.Background(), 0)
	if err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if next != 77 {
		t.Fatalf("cursor = %d, want 77 (reader's next cursor)", next)
	}
}

// TestInitCursor_RetriesHorizonThenSeeds proves the boot cursor is the horizon (not 0) even when
// the first horizon reads fail — so a transient DB error at boot can't trigger a full-outbox replay.
func TestInitCursor_RetriesHorizonThenSeeds(t *testing.T) {
	reader := &horizonReader{horizon: 500, horizonFails: 2}
	d := NewDrainer(reader, &flakyRealtime{}, 0)
	cur, ok := d.initCursor(context.Background())
	if !ok {
		t.Fatal("initCursor gave up")
	}
	if cur != 500 {
		t.Fatalf("init cursor = %d, want 500 (the horizon, retried past 2 failures)", cur)
	}
}

// TestInitCursor_StopsOnCancel proves init doesn't spin forever if ctx is cancelled first.
func TestInitCursor_StopsOnCancel(t *testing.T) {
	reader := &horizonReader{horizonFails: 1_000_000} // never succeeds in time
	d := NewDrainer(reader, &flakyRealtime{}, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := d.initCursor(ctx); ok {
		t.Fatal("initCursor should return ok=false when ctx is already cancelled")
	}
}
