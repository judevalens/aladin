package realtime

import (
	"context"
	"net/url"
	"sort"
	"strings"
)

const (
	WorkspaceStream    = "workspace"
	MarketStream       = "market"
	AnyResource        = "*"
	FrameOperation     = "frame"
	WorkspaceFrameKind = AnyResource + "." + FrameOperation
)

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

type EventService interface {
	Publish(context.Context, PublishTarget, any) error
	Subscribe(context.Context, []SubscriptionKey, string) (<-chan AppEvent, func(), error)
}

type KeyResolver interface {
	ResolveSubscribeKeys(context.Context, SubscriptionOptions) ([]SubscriptionKey, error)
	ResolvePublishKeys(context.Context, PublishTarget) ([]SubscriptionKey, string, error)
}

func (k SubscriptionKey) Matches(other SubscriptionKey) bool {
	if k.Stream != other.Stream {
		return false
	}
	if k.Stream != MarketStream && k.TenantID != other.TenantID {
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
	return PublicSubscriptionKey{EventKind: k.EventKind, Stream: k.Stream, ResourceKind: k.ResourceKind, ResourceID: k.ResourceID, Qualifiers: normalizeQualifiers(k.Qualifiers)}
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
		if key = strings.TrimSpace(key); key != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}
