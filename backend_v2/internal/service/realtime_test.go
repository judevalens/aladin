package service

import (
	"context"
	"errors"
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
	ctx, cancel := context.WithCancel(realtimeTestContext("user-a", ActorTypeUserSession, nil))
	defer cancel()

	resolver := NewSubscriptionKeyResolver()
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
		if event.SubscriptionKey.Stream != WorkspaceStream || event.SubscriptionKey.ResourceKind != "artifact" || event.SubscriptionKey.ResourceID != "artifact-1" {
			t.Fatalf("unexpected public subscription key: %#v", event.SubscriptionKey)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestInMemoryRealtimeSkipsNonMatchingSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(realtimeTestContext("user-a", ActorTypeUserSession, nil))
	defer cancel()

	resolver := NewSubscriptionKeyResolver()
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

func TestInMemoryRealtimeScopesWildcardSubscriptionsToPrincipal(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	realtime := NewInMemoryRealtimeEventService(resolver)
	userACtx := realtimeTestContext("user-a", ActorTypeUserSession, nil)
	userBCtx := realtimeTestContext("user-b", ActorTypeUserSession, nil)

	userAKeys, err := resolver.ResolveSubscribeKeys(userACtx, SubscriptionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	userBKeys, err := resolver.ResolveSubscribeKeys(userBCtx, SubscriptionOptions{})
	if err != nil {
		t.Fatal(err)
	}

	userAEvents, unsubscribeA, err := realtime.Subscribe(userACtx, userAKeys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribeA()
	userBEvents, unsubscribeB, err := realtime.Subscribe(userBCtx, userBKeys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribeB()

	if err := realtime.Publish(userACtx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
		Operation:    "updated",
	}, map[string]string{"id": "artifact-1"}); err != nil {
		t.Fatal(err)
	}

	expectRealtimeEvent(t, userAEvents, "artifact.updated")
	expectNoRealtimeEvent(t, userBEvents)
}

func TestSubscriptionResolverScopesIntegrationTokenToOwner(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	ctx := realtimeTestContext("owner-user", ActorTypeIntegrationToken, []string{ScopeArtifactsRead})

	keys, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("keys length = %d, want 1", len(keys))
	}
	if keys[0].TenantID != "owner-user" {
		t.Fatalf("tenant id = %q, want owner-user", keys[0].TenantID)
	}
	if keys[0].Stream != WorkspaceStream || keys[0].ResourceKind != AnyResource || keys[0].ResourceID != AnyResource {
		t.Fatalf("default key = %#v, want workspace wildcard", keys[0])
	}
}

func TestSubscriptionResolverRejectsIntegrationTokenWithoutArtifactsRead(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	ctx := realtimeTestContext("owner-user", ActorTypeIntegrationToken, []string{ScopeArtifactsWrite})

	_, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ResolveSubscribeKeys error = %v, want ErrForbidden", err)
	}
}

func TestInMemoryRealtimeFiltersByEventKind(t *testing.T) {
	ctx, cancel := context.WithCancel(realtimeTestContext("user-a", ActorTypeUserSession, nil))
	defer cancel()

	resolver := NewSubscriptionKeyResolver()
	realtime := NewInMemoryRealtimeEventService(resolver)
	keys, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:    WorkspaceStream,
			EventKind: "artifact.updated",
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
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
		Operation:    "created",
	}, map[string]string{"id": "artifact-1"}); err != nil {
		t.Fatal(err)
	}
	if err := realtime.Publish(ctx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-2",
		Operation:    "updated",
	}, map[string]string{"id": "artifact-2"}); err != nil {
		t.Fatal(err)
	}

	event := expectRealtimeEvent(t, events, "artifact.updated")
	if event.SubscriptionKey.ResourceID != "artifact-2" {
		t.Fatalf("resource id = %q, want artifact-2", event.SubscriptionKey.ResourceID)
	}
	expectNoRealtimeEvent(t, events)
}

func TestSubscriptionResolverRejectsMalformedEventKind(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	ctx := realtimeTestContext("user-a", ActorTypeUserSession, nil)

	if _, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:    WorkspaceStream,
			EventKind: "artifact",
		}},
	}); err == nil {
		t.Fatal("ResolveSubscribeKeys malformed eventKind error = nil")
	}
	if _, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:    WorkspaceStream,
			EventKind: "bogus.updated",
		}},
	}); err == nil {
		t.Fatal("ResolveSubscribeKeys unknown eventKind prefix error = nil")
	}
	if _, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:    WorkspaceStream,
			EventKind: "*.updated",
		}},
	}); err == nil {
		t.Fatal("ResolveSubscribeKeys wildcard eventKind prefix error = nil")
	}
}

// The live data-event kind "*.frame" must be a valid subscription (the CDC
// drain publishes it cross-resource; the client subscribes to it). This is the
// one wildcard-prefix exception — regression guard for the R1-C live bug where
// the subscribe was rejected and live frames never arrived.
func TestSubscriptionResolverAcceptsFrameEventKind(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	ctx := realtimeTestContext("user-a", ActorTypeUserSession, nil)

	keys, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: AnyResource,
			ResourceID:   AnyResource,
			EventKind:    WorkspaceFrameKind, // "*.frame"
		}},
	})
	if err != nil {
		t.Fatalf("ResolveSubscribeKeys(*.frame) error = %v, want nil", err)
	}
	if len(keys) != 1 || keys[0].EventKind != WorkspaceFrameKind {
		t.Fatalf("keys = %+v, want one with EventKind %q", keys, WorkspaceFrameKind)
	}
}

// End-to-end live-frame delivery: a subscriber on "*.frame" receives a frame the
// drain publishes (Operation: FrameOperation), regardless of the frame's
// resource. Other wildcard kinds (e.g. "*.updated") remain rejected — proven by
// TestSubscriptionResolverRejectsMalformedEventKind.
func TestInMemoryRealtimeDeliversFrameToWildcardSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(realtimeTestContext("user-a", ActorTypeUserSession, nil))
	defer cancel()

	resolver := NewSubscriptionKeyResolver()
	realtime := NewInMemoryRealtimeEventService(resolver)
	keys, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: AnyResource,
			ResourceID:   AnyResource,
			EventKind:    WorkspaceFrameKind,
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
		Stream:       WorkspaceStream,
		ResourceKind: AnyResource,
		ResourceID:   AnyResource,
		Operation:    FrameOperation,
	}, map[string]any{"entities": []any{}}); err != nil {
		t.Fatal(err)
	}

	event := expectRealtimeEvent(t, events, WorkspaceFrameKind)
	if event.Type != WorkspaceFrameKind {
		t.Fatalf("event type = %q, want %q", event.Type, WorkspaceFrameKind)
	}
}

func TestSubscriptionResolverRejectsEventKindResourceMismatch(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	ctx := realtimeTestContext("user-a", ActorTypeUserSession, nil)

	_, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: "folder",
			EventKind:    "artifact.updated",
		}},
	})
	if err == nil {
		t.Fatal("ResolveSubscribeKeys eventKind/resourceKind mismatch error = nil")
	}
}

func TestSubscriptionResolverRejectsUnsupportedPublicSubscriptions(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	ctx := realtimeTestContext("user-a", ActorTypeUserSession, nil)

	if _, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       "tenant",
			ResourceKind: AnyResource,
			ResourceID:   AnyResource,
		}},
	}); err == nil {
		t.Fatal("ResolveSubscribeKeys unsupported stream error = nil")
	}
	if _, err := resolver.ResolveSubscribeKeys(ctx, SubscriptionOptions{
		Subscriptions: []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: "source",
			ResourceID:   AnyResource,
		}},
	}); err == nil {
		t.Fatal("ResolveSubscribeKeys unsupported resource kind error = nil")
	}
}

func TestInMemoryRealtimeReplayUsesPublishedInternalKeys(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	realtime := NewInMemoryRealtimeEventService(resolver)
	userACtx := realtimeTestContext("user-a", ActorTypeUserSession, nil)
	userBCtx := realtimeTestContext("user-b", ActorTypeUserSession, nil)

	userAKeys, err := resolver.ResolveSubscribeKeys(userACtx, SubscriptionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	userAEvents, unsubscribeA, err := realtime.Subscribe(userACtx, userAKeys, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := realtime.Publish(userACtx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-1",
		Operation:    "updated",
	}, map[string]string{"id": "artifact-1"}); err != nil {
		t.Fatal(err)
	}
	first := expectRealtimeEvent(t, userAEvents, "artifact.updated")
	unsubscribeA()

	if err := realtime.Publish(userBCtx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: "artifact",
		ResourceID:   "artifact-b",
		Operation:    "updated",
	}, map[string]string{"id": "artifact-b"}); err != nil {
		t.Fatal(err)
	}
	if err := realtime.Publish(userACtx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: "folder",
		ResourceID:   "folder-a",
		Operation:    "created",
	}, map[string]string{"id": "folder-a"}); err != nil {
		t.Fatal(err)
	}

	replayEvents, unsubscribeReplay, err := realtime.Subscribe(userACtx, userAKeys, first.EventID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribeReplay()

	replayed := expectRealtimeEvent(t, replayEvents, "folder.created")
	if replayed.SubscriptionKey.ResourceID != "folder-a" {
		t.Fatalf("replayed resource id = %q, want folder-a", replayed.SubscriptionKey.ResourceID)
	}
	expectNoRealtimeEvent(t, replayEvents)
}

func expectRealtimeEvent(t *testing.T, events <-chan AppEvent, eventType string) AppEvent {
	t.Helper()
	select {
	case event := <-events:
		if event.Type != eventType {
			t.Fatalf("event type = %q, want %q", event.Type, eventType)
		}
		return event
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s event", eventType)
	}
	return AppEvent{}
}

func expectNoRealtimeEvent(t *testing.T, events <-chan AppEvent) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected event: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func realtimeTestContext(userID string, actorType string, scopes []string) context.Context {
	return WithPrincipal(context.Background(), Principal{
		UserID:    userID,
		ActorType: actorType,
		ActorID:   userID,
		Email:     userID + "@example.test",
		Scopes:    scopes,
	})
}

// A broadcast-stream ('market') event reaches subscribers on DIFFERENT tenants — proving the
// broadcast-scope matcher fans out across users (unlike the tenant-isolated workspace stream).
func TestInMemoryRealtimeBroadcastsMarketToAllTenants(t *testing.T) {
	resolver := NewSubscriptionKeyResolver()
	realtime := NewInMemoryRealtimeEventService(resolver)
	userACtx := realtimeTestContext("user-a", ActorTypeUserSession, nil)
	userBCtx := realtimeTestContext("user-b", ActorTypeUserSession, nil)

	marketSub := SubscriptionOptions{Subscriptions: []PublicSubscriptionKey{
		{Stream: MarketStream, ResourceKind: "quote", ResourceID: "NVDA-id"},
	}}
	aKeys, err := resolver.ResolveSubscribeKeys(userACtx, marketSub)
	if err != nil {
		t.Fatal(err)
	}
	bKeys, err := resolver.ResolveSubscribeKeys(userBCtx, marketSub)
	if err != nil {
		t.Fatal(err)
	}

	aEvents, unsubA, err := realtime.Subscribe(userACtx, aKeys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubA()
	bEvents, unsubB, err := realtime.Subscribe(userBCtx, bKeys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsubB()

	// Broadcast publish (tenant-agnostic) — one write.
	if err := realtime.Publish(context.Background(), PublishTarget{
		Stream:       MarketStream,
		ResourceKind: "quote",
		ResourceID:   "NVDA-id",
		Operation:    "update",
	}, map[string]any{"last": 1183.56}); err != nil {
		t.Fatal(err)
	}

	// Both tenants receive it — fan-out.
	expectRealtimeEvent(t, aEvents, "quote.update")
	expectRealtimeEvent(t, bEvents, "quote.update")
}
