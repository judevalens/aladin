package approval

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeResolver struct {
	turnID, actionID string
	approved         bool
	err              error
}

func (f *fakeResolver) ResolveApproval(_ context.Context, turnID, actionID string, approved bool) error {
	f.turnID, f.actionID, f.approved = turnID, actionID, approved
	return f.err
}

func TestRegistryScopesActionsAndPrunesExpiredEntries(t *testing.T) {
	registry := NewRegistry(time.Minute)
	registry.Register("old", Action{UserID: "u1", Created: time.Now().Add(-2 * time.Minute)})
	registry.Register("live", Action{UserID: "u1", ThreadID: "t1", TurnID: "turn1", Tool: "publish_app"})
	if registry.Count() != 1 {
		t.Fatalf("count = %d, want expired action pruned", registry.Count())
	}
	if _, ok := registry.Take("u2", "live"); ok {
		t.Fatal("wrong owner took action")
	}
	action, ok := registry.Take("u1", "live")
	if !ok || action.TurnID != "turn1" || action.Tool != "publish_app" {
		t.Fatalf("action = %+v, ok=%v", action, ok)
	}
	if registry.Count() != 0 {
		t.Fatal("taken action remains registered")
	}
}

func TestGatewayResolvesThroughProviderAfterOwnerCheck(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("expired")}
	gateway := NewGateway(resolver, time.Minute)
	gateway.Register("a1", Action{UserID: "u1", TurnID: "turn1"})
	if _, found, err := gateway.Resolve(context.Background(), "u2", "a1", true); found || err != nil {
		t.Fatalf("wrong owner found=%v err=%v", found, err)
	}
	action, found, err := gateway.Resolve(context.Background(), "u1", "a1", true)
	if !found || action.TurnID != "turn1" || !errors.Is(err, resolver.err) {
		t.Fatalf("resolve action=%+v found=%v err=%v", action, found, err)
	}
	if resolver.turnID != "turn1" || resolver.actionID != "a1" || !resolver.approved {
		t.Fatalf("provider call = %+v", resolver)
	}
}
