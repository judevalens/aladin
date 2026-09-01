package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"aladin/backend_v2/internal/shardv2"
)

// ResourceTarget is assembled by an authenticated transport, never decoded from
// a shard's contract. Audience is selected by the entry point (app or agent).
type ResourceTarget struct {
	ShardID      string
	Environment  BuildChannel
	Audience     string
	ContractHash string
}

type ResourceRequest struct {
	Binding  string         `json:"binding,omitempty"`
	Resource string         `json:"resource,omitempty"`
	Inputs   map[string]any `json:"inputs,omitempty"`
	Query    *shardv2.Query `json:"query,omitempty"`
	ID       string         `json:"id,omitempty"`
}

type ResourceDescriptor struct {
	Kind          string         `json:"kind"`
	SchemaVersion int64          `json:"schemaVersion"`
	Schema        shardv2.Schema `json:"schema"`
	Capabilities  []string       `json:"capabilities"`
	Observation   string         `json:"observation,omitempty"`
	Delivery      string         `json:"delivery"`
	Limit         int            `json:"limit"`
}

type ResourceSnapshot struct {
	Resource        string           `json:"resource"`
	Records         []shardv2.Record `json:"records"`
	Complete        bool             `json:"complete"`
	NextCursor      string           `json:"nextCursor,omitempty"`
	SourceUpdatedAt string           `json:"sourceUpdatedAt,omitempty"`
}

type ResourceMutation struct {
	ResourceRequest
	Op           string          `json:"op"`
	Data         json.RawMessage `json:"data,omitempty"`
	BaseRevision string          `json:"baseRevision,omitempty"`
	RequestID    string          `json:"requestId"`
}
type ResourceTombstone struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}
type ResourceMutationResult struct {
	RequestID string             `json:"requestId"`
	Record    *shardv2.Record    `json:"record,omitempty"`
	Tombstone *ResourceTombstone `json:"tombstone,omitempty"`
}
type ResourceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ResourceError) Error() string           { return e.Message }
func ResourceFailure(code, message string) error { return &ResourceError{Code: code, Message: message} }
func ResourceErrorCode(err error) string {
	var failure *ResourceError
	if errors.As(err, &failure) {
		return failure.Code
	}
	if errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrForbidden) {
		return "forbidden"
	}
	if errors.Is(err, ErrNotFound) {
		return "not-found"
	}
	return "source-unavailable"
}

// Protected metadata is separate from author-writable artifact metadata/files.
// The build/publish pipeline installs these through protected stage/activation.
type ResourceRelease struct {
	Source     json.RawMessage
	Hash       string
	BuildID    string
	Generation string
}
type ResourceReleaseReader interface {
	ActiveResourceRelease(context.Context, string, string, BuildChannel) (ResourceRelease, error)
}

// These values come from authorization and protected release resolution. They
// are internal adapter arguments and must never be decoded from an HTTP body.
type ResourceNamespace struct {
	UserID       string
	ActorKey     string
	ShardID      string
	Environment  BuildChannel
	DatasetID    string
	Generation   string
	ContractHash string
}
type ResourceView struct {
	Namespace    ResourceNamespace
	Definition   shardv2.Resource
	Params       map[string]any
	Query        shardv2.Query
	ID           string
	URI          string
	ViewHash     string
	Select       []string
	OutputSchema shardv2.Schema
}
type ResourcePage struct {
	Records         []shardv2.Record
	NextCursor      string
	SourceUpdatedAt string
}

type ResourceProvider interface {
	Profile() shardv2.ProviderProfile
	// Authorize must reject parameter escapes and unsupported declarations. It
	// runs for every read/write/refresh, not just subscription setup.
	Authorize(context.Context, ResourceView) error
	Snapshot(context.Context, ResourceView) (ResourcePage, error)
	Mutate(context.Context, ResourceView, shardv2.Command) (ResourceMutationResult, error)
}

type ResourceSubscriptionIdentity struct {
	SubscriptionID string `json:"subscriptionId"`
	Resource       string `json:"resource"`
	Epoch          string `json:"epoch"`
}
type ResourceStreamMessage struct {
	Event *shardv2.Event
	Error *ResourceError
}
type ResourceSubscription struct {
	Identity ResourceSubscriptionIdentity
	Events   <-chan ResourceStreamMessage
	Close    func()
}

type ShardResourceService interface {
	Hello(context.Context, ResourceTarget) (map[string]any, error)
	Describe(context.Context, ResourceTarget, ResourceRequest) (ResourceDescriptor, error)
	Read(context.Context, ResourceTarget, ResourceRequest) (ResourceSnapshot, error)
	Mutate(context.Context, ResourceTarget, ResourceMutation) (ResourceMutationResult, error)
	Subscribe(context.Context, ResourceTarget, ResourceRequest) (ResourceSubscription, error)
}

type ResourceServiceOptions struct {
	RefreshInterval              time.Duration
	MaxSubscriptionsPerPrincipal int
	RequestsPerSecond            int
	RequestBurst                 int
}
