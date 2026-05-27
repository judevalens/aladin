package service

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	WorkspaceStream = "workspace"
	AnyResource     = "*"
)

var allowedWorkspaceResourceKinds = map[string]bool{
	AnyResource: true,
	"artifact":  true,
	"folder":    true,
	"page":      true,
}

type SubscriptionKey struct {
	TenantID     string            `json:"tenantId,omitempty"`
	EventKind    string            `json:"eventKind,omitempty"`
	Stream       string            `json:"stream"`
	ResourceKind string            `json:"resourceKind"`
	ResourceID   string            `json:"resourceId"`
	Qualifiers   map[string]string `json:"qualifiers,omitempty"`
}

type PublicSubscriptionKey struct {
	EventKind    string            `json:"eventKind,omitempty"`
	Stream       string            `json:"stream"`
	ResourceKind string            `json:"resourceKind"`
	ResourceID   string            `json:"resourceId"`
	Qualifiers   map[string]string `json:"qualifiers,omitempty"`
}

type PublishTarget struct {
	TenantID     string
	Stream       string
	ResourceKind string
	ResourceID   string
	Operation    string
	Qualifiers   map[string]string
}

type SubscriptionOptions struct {
	Subscriptions []PublicSubscriptionKey `json:"subscriptions"`
}

type AppEvent struct {
	EventID         string                `json:"eventId"`
	Type            string                `json:"type"`
	SubscriptionKey PublicSubscriptionKey `json:"subscriptionKey"`
	Payload         any                   `json:"payload"`
	OccurredAt      string                `json:"occurredAt"`
}

type RealtimeEventService interface {
	Publish(context.Context, PublishTarget, any) error
	Subscribe(context.Context, []SubscriptionKey, string) (<-chan AppEvent, func(), error)
}

type SubscriptionKeyResolver interface {
	ResolveSubscribeKeys(context.Context, SubscriptionOptions) ([]SubscriptionKey, error)
	ResolvePublishKeys(context.Context, PublishTarget) ([]SubscriptionKey, string, error)
}

type DefaultSubscriptionKeyResolver struct{}

func NewSubscriptionKeyResolver() *DefaultSubscriptionKeyResolver {
	return &DefaultSubscriptionKeyResolver{}
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
		subscriptions = []PublicSubscriptionKey{{
			Stream:       WorkspaceStream,
			ResourceKind: AnyResource,
			ResourceID:   AnyResource,
		}}
	}

	keys := make([]SubscriptionKey, 0, len(subscriptions))
	for _, sub := range subscriptions {
		stream := defaultString(sub.Stream, WorkspaceStream)
		resourceKind := defaultString(sub.ResourceKind, AnyResource)
		resourceID := defaultString(sub.ResourceID, AnyResource)
		eventKind := strings.TrimSpace(sub.EventKind)
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
		key := SubscriptionKey{
			TenantID:     principal.UserID,
			EventKind:    eventKind,
			Stream:       stream,
			ResourceKind: resourceKind,
			ResourceID:   resourceID,
			Qualifiers:   normalizeQualifiers(sub.Qualifiers),
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func validateWorkspaceEventKind(eventKind, resourceKind string) error {
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
	tenantID := strings.TrimSpace(target.TenantID)
	if tenantID == "" {
		principal, err := RequirePrincipal(ctx)
		if err != nil {
			return nil, "", err
		}
		tenantID = principal.UserID
	}
	stream := defaultString(target.Stream, WorkspaceStream)
	resourceKind := defaultString(target.ResourceKind, AnyResource)
	resourceID := defaultString(target.ResourceID, AnyResource)
	if stream != WorkspaceStream {
		return nil, "", BadRequest("unsupported realtime stream")
	}
	if !allowedWorkspaceResourceKinds[resourceKind] {
		return nil, "", BadRequest("unsupported workspace resource kind")
	}
	operation := strings.TrimSpace(target.Operation)
	if operation == "" {
		operation = "updated"
	}
	eventType := resourceKind + "." + operation
	qualifiers := normalizeQualifiers(target.Qualifiers)

	return []SubscriptionKey{
		{
			TenantID:     tenantID,
			EventKind:    eventType,
			Stream:       stream,
			ResourceKind: resourceKind,
			ResourceID:   resourceID,
			Qualifiers:   qualifiers,
		},
		{
			TenantID:     tenantID,
			EventKind:    eventType,
			Stream:       stream,
			ResourceKind: AnyResource,
			ResourceID:   AnyResource,
		},
	}, eventType, nil
}

type InMemoryRealtimeEventService struct {
	resolver     SubscriptionKeyResolver
	mu           sync.RWMutex
	nextID       int64
	subscribers  map[string]realtimeSubscriber
	recentEvents []recentRealtimeEvent
	recentLimit  int
}

type realtimeSubscriber struct {
	keys []SubscriptionKey
	ch   chan AppEvent
	done chan struct{}
}

type recentRealtimeEvent struct {
	event AppEvent
	keys  []SubscriptionKey
}

func NewInMemoryRealtimeEventService(resolver SubscriptionKeyResolver) *InMemoryRealtimeEventService {
	return &InMemoryRealtimeEventService{
		resolver:    resolver,
		subscribers: make(map[string]realtimeSubscriber),
		recentLimit: 256,
	}
}

func (s *InMemoryRealtimeEventService) Publish(ctx context.Context, target PublishTarget, payload any) error {
	keys, eventType, err := s.resolver.ResolvePublishKeys(ctx, target)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	event := AppEvent{
		EventID:         "evt-" + uuid.NewString(),
		Type:            eventType,
		SubscriptionKey: keys[0].Public(),
		Payload:         payload,
		OccurredAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}

	s.mu.Lock()
	s.recentEvents = append(s.recentEvents, recentRealtimeEvent{event: event, keys: cloneSubscriptionKeys(keys)})
	if len(s.recentEvents) > s.recentLimit {
		s.recentEvents = s.recentEvents[len(s.recentEvents)-s.recentLimit:]
	}

	delivered := make(map[string]bool)
	for subscriberID, sub := range s.subscribers {
		if !subscriberMatchesAny(sub.keys, keys) || delivered[subscriberID] {
			continue
		}
		delivered[subscriberID] = true
		select {
		case sub.ch <- event:
		default:
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *InMemoryRealtimeEventService) Subscribe(ctx context.Context, keys []SubscriptionKey, afterEventID string) (<-chan AppEvent, func(), error) {
	id := uuid.NewString()
	ch := make(chan AppEvent, 64)
	done := make(chan struct{})

	s.mu.Lock()
	s.subscribers[id] = realtimeSubscriber{keys: keys, ch: ch, done: done}
	replay := s.replayLocked(keys, afterEventID)
	s.mu.Unlock()

	go func() {
		for _, event := range replay {
			select {
			case ch <- event:
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	unsubscribe := func() {
		s.mu.Lock()
		if sub, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(sub.done)
		}
		s.mu.Unlock()
	}
	return ch, unsubscribe, nil
}

func (s *InMemoryRealtimeEventService) replayLocked(keys []SubscriptionKey, afterEventID string) []AppEvent {
	if afterEventID == "" {
		return nil
	}
	found := false
	out := make([]AppEvent, 0)
	for _, event := range s.recentEvents {
		if !found {
			found = event.event.EventID == afterEventID
			continue
		}
		if subscriberMatchesAny(keys, event.keys) {
			out = append(out, event.event)
		}
	}
	return out
}

func cloneSubscriptionKeys(keys []SubscriptionKey) []SubscriptionKey {
	out := make([]SubscriptionKey, 0, len(keys))
	for _, key := range keys {
		key.Qualifiers = normalizeQualifiers(key.Qualifiers)
		out = append(out, key)
	}
	return out
}

func subscriberMatchesAny(subscriberKeys []SubscriptionKey, publishedKeys []SubscriptionKey) bool {
	for _, sub := range subscriberKeys {
		for _, pub := range publishedKeys {
			if sub.Matches(pub) {
				return true
			}
		}
	}
	return false
}

func (k SubscriptionKey) Matches(other SubscriptionKey) bool {
	if k.TenantID != other.TenantID || k.Stream != other.Stream {
		return false
	}
	if k.EventKind != "" && k.EventKind != other.EventKind {
		return false
	}
	if k.ResourceKind != AnyResource && k.ResourceKind != other.ResourceKind {
		return false
	}
	if k.ResourceID != AnyResource && k.ResourceID != other.ResourceID {
		return false
	}
	return CanonicalQualifiers(k.Qualifiers) == CanonicalQualifiers(other.Qualifiers)
}

func (k SubscriptionKey) Public() PublicSubscriptionKey {
	return PublicSubscriptionKey{
		EventKind:    k.EventKind,
		Stream:       k.Stream,
		ResourceKind: k.ResourceKind,
		ResourceID:   k.ResourceID,
		Qualifiers:   normalizeQualifiers(k.Qualifiers),
	}
}

func CanonicalQualifiers(q map[string]string) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for key := range q {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(q[key]))
	}
	return strings.Join(parts, "&")
}

func normalizeQualifiers(q map[string]string) map[string]string {
	if len(q) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(q))
	for key, value := range q {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		out[trimmedKey] = strings.TrimSpace(value)
	}
	return out
}

func defaultString(value string, fallback string) string {
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
