package service

import (
	"context"
	"encoding/json"
	"strings"

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

var allowedWorkspaceResourceKinds = map[string]bool{
	AnyResource: true, "artifact": true, "folder": true, "page": true,
	"copilot": true, "notification": true,
}

var broadcastStreams = map[string]map[string]bool{
	MarketStream: {AnyResource: true, "quote": true},
}

func isBroadcastStream(stream string) bool { _, ok := broadcastStreams[stream]; return ok }

type DefaultSubscriptionKeyResolver struct{}

func NewSubscriptionKeyResolver() *DefaultSubscriptionKeyResolver {
	return &DefaultSubscriptionKeyResolver{}
}

// NewInMemoryRealtimeEventService is a source-compatible construction shim.
// Application composition uses realtime.NewService directly; the concrete owner is realtime.
func NewInMemoryRealtimeEventService(resolver SubscriptionKeyResolver) *runtime.Service {
	return runtime.NewService(resolver)
}

func (r *DefaultSubscriptionKeyResolver) ResolveSubscribeKeys(ctx context.Context, opts SubscriptionOptions) ([]SubscriptionKey, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	subscriptions := opts.Subscriptions
	if len(subscriptions) == 0 {
		subscriptions = []PublicSubscriptionKey{{Stream: WorkspaceStream, ResourceKind: AnyResource, ResourceID: AnyResource}}
	}
	keys := make([]SubscriptionKey, 0, len(subscriptions))
	for _, sub := range subscriptions {
		stream := defaultString(sub.Stream, WorkspaceStream)
		resourceKind := defaultString(sub.ResourceKind, AnyResource)
		resourceID := defaultString(sub.ResourceID, AnyResource)
		eventKind := strings.TrimSpace(sub.EventKind)
		if isBroadcastStream(stream) {
			if !broadcastStreams[stream][resourceKind] {
				return nil, BadRequest("unsupported broadcast resource kind")
			}
			keys = append(keys, SubscriptionKey{EventKind: eventKind, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: normalizeQualifiers(sub.Qualifiers)})
			continue
		}
		if stream != WorkspaceStream {
			return nil, BadRequest("unsupported realtime stream")
		}
		if !allowedWorkspaceResourceKinds[resourceKind] {
			return nil, BadRequest("unsupported workspace resource kind")
		}
		if eventKind != "" {
			if err := validateWorkspaceEventKind(eventKind, resourceKind); err != nil {
				return nil, err
			}
		}
		keys = append(keys, SubscriptionKey{TenantID: principal.UserID, EventKind: eventKind, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: normalizeQualifiers(sub.Qualifiers)})
	}
	return keys, nil
}

func validateWorkspaceEventKind(eventKind, resourceKind string) error {
	if eventKind == WorkspaceFrameKind {
		return nil
	}
	parts := strings.SplitN(eventKind, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return BadRequest("eventKind must be in the form 'resource.operation'")
	}
	prefix := parts[0]
	if prefix == AnyResource || !allowedWorkspaceResourceKinds[prefix] {
		return BadRequest("unsupported eventKind resource prefix")
	}
	if resourceKind != AnyResource && resourceKind != prefix {
		return BadRequest("eventKind does not match resourceKind")
	}
	return nil
}

func (r *DefaultSubscriptionKeyResolver) ResolvePublishKeys(ctx context.Context, target PublishTarget) ([]SubscriptionKey, string, error) {
	stream := defaultString(target.Stream, WorkspaceStream)
	resourceKind := defaultString(target.ResourceKind, AnyResource)
	resourceID := defaultString(target.ResourceID, AnyResource)
	operation := strings.TrimSpace(target.Operation)
	if operation == "" {
		operation = "updated"
	}
	qualifiers := normalizeQualifiers(target.Qualifiers)
	if isBroadcastStream(stream) {
		if !broadcastStreams[stream][resourceKind] {
			return nil, "", BadRequest("unsupported broadcast resource kind")
		}
		eventType := resourceKind + "." + operation
		return []SubscriptionKey{
			{EventKind: eventType, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: qualifiers},
			{EventKind: eventType, Stream: stream, ResourceKind: AnyResource, ResourceID: AnyResource},
		}, eventType, nil
	}
	tenantID := strings.TrimSpace(target.TenantID)
	if tenantID == "" {
		principal, err := RequirePrincipal(ctx)
		if err != nil {
			return nil, "", err
		}
		tenantID = principal.UserID
	}
	if stream != WorkspaceStream {
		return nil, "", BadRequest("unsupported realtime stream")
	}
	if !allowedWorkspaceResourceKinds[resourceKind] {
		return nil, "", BadRequest("unsupported workspace resource kind")
	}
	eventType := resourceKind + "." + operation
	return []SubscriptionKey{
		{TenantID: tenantID, EventKind: eventType, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: qualifiers},
		{TenantID: tenantID, EventKind: eventType, Stream: stream, ResourceKind: AnyResource, ResourceID: AnyResource},
	}, eventType, nil
}

func CanonicalQualifiers(q map[string]string) string { return runtime.CanonicalQualifiers(q) }

func normalizeQualifiers(q map[string]string) map[string]string {
	if len(q) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(q))
	for key, value := range q {
		if key = strings.TrimSpace(key); key != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

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
