package changefeed

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
	. "aladin/backend_v2/internal/realtime"
	. "aladin/backend_v2/internal/service"
)

type fakeDrainReader struct {
	events  []DrainedEvent
	horizon uint64
}

func (f *fakeDrainReader) DrainSince(_ context.Context, _ uint64) ([]DrainedEvent, uint64, error) {
	return f.events, f.horizon, nil
}

func (f *fakeDrainReader) Horizon(_ context.Context) (uint64, error) {
	return f.horizon, nil
}

// drainOnce publishes each event's frame to ITS user's subscribers (TenantID =
// event.UserID), tagged `*.frame`, and returns the horizon as the new cursor.
func TestOutboxDrain_PublishesFramePerUser(t *testing.T) {
	resolver := newTestSubscriptionKeyResolver()
	rt := NewService(resolver)

	ch, unsub, err := rt.Subscribe(context.Background(), []SubscriptionKey{{
		TenantID: "u1", Stream: WorkspaceStream, ResourceKind: AnyResource, ResourceID: AnyResource,
	}}, "")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	frame := Frame{Entities: []FrameEntity{{EntityKind: "folder", EntityID: "f1", Seq: 1, Op: OpUpsert}}}
	reader := &fakeDrainReader{
		events:  []DrainedEvent{{Xid: 42, UserID: "u1", Frame: frame}},
		horizon: 100,
	}
	drainer := NewDrainer(reader, rt, 0)

	next, err := drainer.drainOnce(context.Background(), 0)
	if err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if next != 100 {
		t.Fatalf("cursor = %d, want 100 (horizon)", next)
	}

	select {
	case ev := <-ch:
		if ev.Type != "*.frame" {
			t.Fatalf("event type = %q, want *.frame", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame published to u1 within 2s")
	}
}

// A frame for user u2 must NOT reach user u1's subscriber.
func TestOutboxDrain_PerUserIsolation(t *testing.T) {
	resolver := newTestSubscriptionKeyResolver()
	rt := NewService(resolver)

	ch, unsub, err := rt.Subscribe(context.Background(), []SubscriptionKey{{
		TenantID: "u1", Stream: WorkspaceStream, ResourceKind: AnyResource, ResourceID: AnyResource,
	}}, "")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	reader := &fakeDrainReader{
		events: []DrainedEvent{{Xid: 7, UserID: "u2", Frame: Frame{
			Entities: []FrameEntity{{EntityKind: "folder", EntityID: "x", Seq: 1, Op: OpUpsert}},
		}}},
		horizon: 9,
	}
	drainer := NewDrainer(reader, rt, 0)
	if _, err := drainer.drainOnce(context.Background(), 0); err != nil {
		t.Fatalf("drainOnce: %v", err)
	}

	select {
	case ev := <-ch:
		t.Fatalf("u1 received a frame meant for u2: %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// expected: nothing
	}
}

func newTestSubscriptionKeyResolver() *SubscriptionKeyResolver {
	return NewSubscriptionKeyResolver(
		func(ctx context.Context) (string, error) {
			principal, err := auth.RequirePrincipal(ctx)
			return principal.UserID, err
		},
		func(ctx context.Context) error { return auth.RequireScope(ctx, auth.ScopeArtifactsRead) },
		func(message string) error { return apperror.BadRequest(message) },
	)
}
