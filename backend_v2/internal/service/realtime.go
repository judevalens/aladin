package service

import (
	"context"
	"encoding/json"

	runtime "aladin/backend_v2/internal/realtime"
)

const (
	WorkspaceStream    = runtime.WorkspaceStream
	MarketStream       = runtime.MarketStream
	AnyResource        = runtime.AnyResource
	FrameOperation     = runtime.FrameOperation
	WorkspaceFrameKind = runtime.WorkspaceFrameKind
)

type SubscriptionKey = runtime.SubscriptionKey
type PublicSubscriptionKey = runtime.PublicSubscriptionKey
type PublishTarget = runtime.PublishTarget
type SubscriptionOptions = runtime.SubscriptionOptions
type AppEvent = runtime.AppEvent
type RealtimeEventService = runtime.EventService
type SubscriptionKeyResolver = runtime.KeyResolver
type InMemoryRealtimeEventService = runtime.Service

type DefaultSubscriptionKeyResolver = runtime.SubscriptionKeyResolver

func NewSubscriptionKeyResolver() *runtime.SubscriptionKeyResolver {
	return runtime.NewSubscriptionKeyResolver(
		func(ctx context.Context) (string, error) {
			principal, err := RequirePrincipal(ctx)
			return principal.UserID, err
		},
		func(ctx context.Context) error { return RequireScope(ctx, ScopeArtifactsRead) },
		func(message string) error { return BadRequest(message) },
	)
}

// NewInMemoryRealtimeEventService is a source-compatible construction shim.
// Application composition uses realtime.NewService directly; the concrete owner is realtime.
func NewInMemoryRealtimeEventService(resolver SubscriptionKeyResolver) *runtime.Service {
	return runtime.NewService(resolver)
}

func CanonicalQualifiers(q map[string]string) string { return runtime.CanonicalQualifiers(q) }

func EventPayload(value any) any {
	bytes, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var out any
	if err := json.Unmarshal(bytes, &out); err != nil {
		return value
	}
	return out
}
