package service

import (
	"context"
	"errors"
	"testing"
)

// Unit tests for the delta-vs-snapshot policy in DefaultSyncService, using fake
// ports (no database). These exercise the routing rules in §6 of the design.

type fakeOutbox struct {
	frames     []Frame
	horizon    uint64
	minXid     uint64
	hasMin     bool
	pullCalled bool
	pullCursor uint64
	err        error
}

func (f *fakeOutbox) PullSince(_ context.Context, _ string, cursor uint64) ([]Frame, uint64, error) {
	f.pullCalled = true
	f.pullCursor = cursor
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.frames, f.horizon, nil
}

func (f *fakeOutbox) MinXid(_ context.Context, _ string) (uint64, bool, error) {
	if f.err != nil {
		return 0, false, f.err
	}
	return f.minXid, f.hasMin, nil
}

func (f *fakeOutbox) Horizon(_ context.Context) (uint64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.horizon, nil
}

type fakeSource struct {
	kind     string
	entities []FrameEntity
	called   bool
	err      error
}

func (s *fakeSource) EntityKind() string { return s.kind }

func (s *fakeSource) Snapshot(_ context.Context, _ string) ([]FrameEntity, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.entities, nil
}

// cursor == 0 → snapshot (cold client), gathering every source's entities and
// resuming at the horizon. The outbox delta path is NOT consulted.
func TestSyncService_ColdCursorSnapshots(t *testing.T) {
	ob := &fakeOutbox{horizon: 42}
	src := &fakeSource{kind: "tree", entities: []FrameEntity{
		{EntityKind: "folder", EntityID: "f1", Seq: 1, Op: OpUpsert},
		{EntityKind: "folder", EntityID: "f2", Seq: 3, Op: OpDelete},
	}}
	svc := NewSyncService(ob, src)

	res, err := svc.Pull(context.Background(), "u1", 0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if res.Mode != PullModeSnapshot {
		t.Fatalf("mode = %q, want snapshot", res.Mode)
	}
	if !src.called {
		t.Fatalf("snapshot source was not consulted")
	}
	if ob.pullCalled {
		t.Fatalf("delta path was consulted on a cold pull")
	}
	if res.Cursor != 42 {
		t.Fatalf("cursor = %d, want 42 (horizon)", res.Cursor)
	}
	if n := countEntities(res.Frames); n != 2 {
		t.Fatalf("snapshot entity count = %d, want 2", n)
	}
}

// A live cursor within retention → delta path; the cursor is passed through and
// the horizon comes back as the new cursor.
func TestSyncService_LiveCursorDeltas(t *testing.T) {
	ob := &fakeOutbox{
		horizon: 100,
		minXid:  5, hasMin: true,
		frames: []Frame{{Entities: []FrameEntity{{EntityKind: "folder", EntityID: "f1", Seq: 2, Op: OpUpsert}}}},
	}
	svc := NewSyncService(ob, &fakeSource{kind: "tree"})

	res, err := svc.Pull(context.Background(), "u1", 50)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if res.Mode != PullModeDelta {
		t.Fatalf("mode = %q, want delta", res.Mode)
	}
	if !ob.pullCalled || ob.pullCursor != 50 {
		t.Fatalf("delta pull cursor = %d (called=%v), want 50", ob.pullCursor, ob.pullCalled)
	}
	if res.Cursor != 100 {
		t.Fatalf("cursor = %d, want 100 (horizon)", res.Cursor)
	}
}

// A cursor that has fallen behind the oldest retained xid → snapshot (else the
// client would silently miss pruned events).
func TestSyncService_CursorBehindRetentionSnapshots(t *testing.T) {
	ob := &fakeOutbox{horizon: 100, minXid: 20, hasMin: true}
	src := &fakeSource{kind: "tree", entities: []FrameEntity{{EntityKind: "folder", EntityID: "f1", Seq: 1, Op: OpUpsert}}}
	svc := NewSyncService(ob, src)

	res, err := svc.Pull(context.Background(), "u1", 10) // 10 < minXid 20
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if res.Mode != PullModeSnapshot {
		t.Fatalf("mode = %q, want snapshot (cursor behind retention)", res.Mode)
	}
	if !src.called {
		t.Fatalf("snapshot source not consulted")
	}
}

// An empty snapshot returns no frames (not a nil-deref) and still advances to
// the horizon.
func TestSyncService_EmptySnapshot(t *testing.T) {
	ob := &fakeOutbox{horizon: 7}
	svc := NewSyncService(ob, &fakeSource{kind: "tree"})

	res, err := svc.Pull(context.Background(), "u1", 0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if res.Mode != PullModeSnapshot || res.Cursor != 7 {
		t.Fatalf("res = %+v, want snapshot cursor 7", res)
	}
	if len(res.Frames) != 0 {
		t.Fatalf("frames = %d, want 0 for empty snapshot", len(res.Frames))
	}
}

// Errors from the ports propagate.
func TestSyncService_PropagatesErrors(t *testing.T) {
	sentinel := errors.New("boom")
	svc := NewSyncService(&fakeOutbox{err: sentinel}, &fakeSource{kind: "tree", err: sentinel})
	if _, err := svc.Pull(context.Background(), "u1", 0); !errors.Is(err, sentinel) {
		t.Fatalf("cold pull err = %v, want sentinel", err)
	}
	if _, err := svc.Pull(context.Background(), "u1", 99); !errors.Is(err, sentinel) {
		t.Fatalf("delta pull err = %v, want sentinel", err)
	}
}

func countEntities(frames []Frame) int {
	n := 0
	for _, f := range frames {
		n += len(f.Entities)
	}
	return n
}
