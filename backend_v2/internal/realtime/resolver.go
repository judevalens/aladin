package realtime

import (
	"context"
	"strings"
)

type PrincipalUserID func(context.Context) (string, error)
type RequireReadAccess func(context.Context) error
type InvalidRequest func(string) error

type SubscriptionKeyResolver struct {
	principalUserID   PrincipalUserID
	requireReadAccess RequireReadAccess
	invalidRequest    InvalidRequest
}

func NewSubscriptionKeyResolver(principalUserID PrincipalUserID, requireReadAccess RequireReadAccess, invalidRequest InvalidRequest) *SubscriptionKeyResolver {
	return &SubscriptionKeyResolver{
		principalUserID:   principalUserID,
		requireReadAccess: requireReadAccess,
		invalidRequest:    invalidRequest,
	}
}

var allowedWorkspaceResourceKinds = map[string]bool{
	AnyResource: true, "artifact": true, "folder": true, "page": true,
	"copilot": true, "notification": true,
}

var broadcastStreams = map[string]map[string]bool{
	MarketStream: {AnyResource: true, "quote": true},
}

func isBroadcastStream(stream string) bool { _, ok := broadcastStreams[stream]; return ok }

func (r *SubscriptionKeyResolver) ResolveSubscribeKeys(ctx context.Context, opts SubscriptionOptions) ([]SubscriptionKey, error) {
	userID, err := r.principalUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.requireReadAccess(ctx); err != nil {
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
				return nil, r.badRequest("unsupported broadcast resource kind")
			}
			keys = append(keys, SubscriptionKey{EventKind: eventKind, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: normalizeQualifiers(sub.Qualifiers)})
			continue
		}
		if stream != WorkspaceStream {
			return nil, r.badRequest("unsupported realtime stream")
		}
		if !allowedWorkspaceResourceKinds[resourceKind] {
			return nil, r.badRequest("unsupported workspace resource kind")
		}
		if eventKind != "" {
			if err := r.validateWorkspaceEventKind(eventKind, resourceKind); err != nil {
				return nil, err
			}
		}
		keys = append(keys, SubscriptionKey{TenantID: userID, EventKind: eventKind, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: normalizeQualifiers(sub.Qualifiers)})
	}
	return keys, nil
}

func (r *SubscriptionKeyResolver) validateWorkspaceEventKind(eventKind, resourceKind string) error {
	if eventKind == WorkspaceFrameKind {
		return nil
	}
	parts := strings.SplitN(eventKind, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return r.badRequest("eventKind must be in the form 'resource.operation'")
	}
	prefix := parts[0]
	if prefix == AnyResource || !allowedWorkspaceResourceKinds[prefix] {
		return r.badRequest("unsupported eventKind resource prefix")
	}
	if resourceKind != AnyResource && resourceKind != prefix {
		return r.badRequest("eventKind does not match resourceKind")
	}
	return nil
}

func (r *SubscriptionKeyResolver) ResolvePublishKeys(ctx context.Context, target PublishTarget) ([]SubscriptionKey, string, error) {
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
			return nil, "", r.badRequest("unsupported broadcast resource kind")
		}
		eventType := resourceKind + "." + operation
		return []SubscriptionKey{
			{EventKind: eventType, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: qualifiers},
			{EventKind: eventType, Stream: stream, ResourceKind: AnyResource, ResourceID: AnyResource},
		}, eventType, nil
	}
	tenantID := strings.TrimSpace(target.TenantID)
	if tenantID == "" {
		var err error
		tenantID, err = r.principalUserID(ctx)
		if err != nil {
			return nil, "", err
		}
	}
	if stream != WorkspaceStream {
		return nil, "", r.badRequest("unsupported realtime stream")
	}
	if !allowedWorkspaceResourceKinds[resourceKind] {
		return nil, "", r.badRequest("unsupported workspace resource kind")
	}
	eventType := resourceKind + "." + operation
	return []SubscriptionKey{
		{TenantID: tenantID, EventKind: eventType, Stream: stream, ResourceKind: resourceKind, ResourceID: resourceID, Qualifiers: qualifiers},
		{TenantID: tenantID, EventKind: eventType, Stream: stream, ResourceKind: AnyResource, ResourceID: AnyResource},
	}, eventType, nil
}

func (r *SubscriptionKeyResolver) badRequest(message string) error {
	if r.invalidRequest == nil {
		return invalidRequestError(message)
	}
	return r.invalidRequest(message)
}

type invalidRequestError string

func (e invalidRequestError) Error() string { return string(e) }

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
