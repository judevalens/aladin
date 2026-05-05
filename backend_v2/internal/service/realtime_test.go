package service

import (
	"context"
	"testing"
	"time"
)

func TestCanonicalQualifiersSortsKeys(t *testing.T) {
	left := CanonicalQualifiers(map[string]string{
		"market":   "NASDAQ",
		"currency": "USD",
	})
	right := CanonicalQualifiers(map[string]string{
		"currency": "USD",
		"market":   "NASDAQ",
	})

	if left != right {
		t.Fatalf("expected canonical qualifiers to match, got %q and %q", left, right)
	}
}

func TestInMemoryRealtimePublishesToMatchingSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := NewSubscriptionKeyResolver("tenant-1")
	realtime := NewInMemoryRealtimeEventService(resolver)
	keys, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: "artifact",
			ResourceID:   "artifact-1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, unsubscribe, err := realtime.Subscribe(ctx, keys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	if err := realtime.Publish(ctx, PublishTarget{
		TenantID:     "tenant-1",
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
		Operation:    "updated",
	}, map[string]string{"id": "artifact-1"}); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-events:
		if event.Type != "artifact.updated" {
			t.Fatalf("expected artifact.updated event, got %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestInMemoryRealtimeSkipsNonMatchingSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := NewSubscriptionKeyResolver("tenant-1")
	realtime := NewInMemoryRealtimeEventService(resolver)
	keys, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: "artifact",
			ResourceID:   "artifact-other",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, unsubscribe, err := realtime.Subscribe(ctx, keys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	if err := realtime.Publish(ctx, PublishTarget{
		TenantID:     "tenant-1",
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
		Operation:    "updated",
	}, map[string]string{"id": "artifact-1"}); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-events:
		t.Fatalf("unexpected event: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}
