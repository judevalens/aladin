package changefeed

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/reconciliation"
	. "aladin/backend_v2/internal/service"
)

// A websocket publication failure is a latency failure, not a durability failure:
// the client's next pull still converges from the committed outbox frame.
func TestRealtimeLossIsHealedByDurablePull(t *testing.T) {
	frame := Frame{Entities: []FrameEntity{{EntityKind: "folder", EntityID: "f1", Seq: 2, Op: OpUpsert}}}
	store := &convergenceStore{frame: frame}
	drainer := NewDrainer(store, &flakyRealtime{failAt: 1}, 0)

	next, err := drainer.drainOnce(context.Background(), 10)
	if err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if next != 11 {
		t.Fatalf("failed live cursor = %d, want retry xid 11", next)
	}

	pull := reconciliation.New(store)
	result, err := pull.Pull(context.Background(), "u1", 10)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if result.Mode != PullModeDelta || result.Cursor != 12 || len(result.Frames) != 1 {
		t.Fatalf("pull result = %+v, want one delta frame at cursor 12", result)
	}
	if got := result.Frames[0].Entities[0]; got.EntityID != "f1" || got.Seq != 2 {
		t.Fatalf("healed entity = %+v, want f1 seq 2", got)
	}
}

type convergenceStore struct{ frame Frame }

func (s *convergenceStore) DrainSince(context.Context, uint64) ([]DrainedEvent, uint64, error) {
	return []DrainedEvent{{Xid: 11, UserID: "u1", Frame: s.frame}}, 12, nil
}

func (s *convergenceStore) PullSince(context.Context, string, uint64) ([]Frame, uint64, error) {
	return []Frame{s.frame}, 12, nil
}

func (s *convergenceStore) MinXid(context.Context, string) (uint64, bool, error) {
	return 1, true, nil
}

func (s *convergenceStore) Horizon(context.Context) (uint64, error) { return 12, nil }
