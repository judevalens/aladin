// Package shardresource owns the application-facing contracts for Shard v2
// resources. Protocol validation remains in shardv2; transports and storage
// depend on these ports instead of defining their own resource semantics.
package shardresource

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"aladin/backend_v2/internal/shardv2"
)

type Environment string

const (
	EnvironmentPublished Environment = "published"
	EnvironmentDraft     Environment = "draft"
	ChannelPublished                 = EnvironmentPublished
	ChannelDraft                     = EnvironmentDraft
)

// Target is assembled by an authenticated transport, never decoded from a
// shard contract. Audience is selected by the entry point (app or agent).
type Target struct {
	ShardID      string
	Environment  Environment
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

type Descriptor struct {
	Kind          string         `json:"kind"`
	SchemaVersion int64          `json:"schemaVersion"`
	Schema        shardv2.Schema `json:"schema"`
	Capabilities  []string       `json:"capabilities"`
	Observation   string         `json:"observation,omitempty"`
	Delivery      string         `json:"delivery"`
	Limit         int            `json:"limit"`
}

type Snapshot struct {
	Resource        string           `json:"resource"`
	Records         []shardv2.Record `json:"records"`
	Complete        bool             `json:"complete"`
	NextCursor      string           `json:"nextCursor,omitempty"`
	SourceUpdatedAt string           `json:"sourceUpdatedAt,omitempty"`
}

type Mutation struct {
	ResourceRequest
	Op           string          `json:"op"`
	Data         json.RawMessage `json:"data,omitempty"`
	BaseRevision string          `json:"baseRevision,omitempty"`
	RequestID    string          `json:"requestId"`
}

type Tombstone struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

type MutationResult struct {
	RequestID string          `json:"requestId"`
	Record    *shardv2.Record `json:"record,omitempty"`
	Tombstone *Tombstone      `json:"tombstone,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string           { return e.Message }
func Failure(code, message string) error { return &Error{Code: code, Message: message} }
func ResourceFailure(code, message string) error {
	return Failure(code, message)
}
func ErrorCode(err error, fallback string) string {
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code
	}
	return fallback
}

// Release is protected metadata installed only by stage/activation.
type ResourceRelease struct {
	Source     json.RawMessage
	Hash       string
	BuildID    string
	Generation string
}

type ReleaseReader interface {
	ActiveResourceRelease(context.Context, string, string, Environment) (ResourceRelease, error)
}

// Namespace fields are derived from authorization and protected release state.
type Namespace struct {
	UserID       string
	ActorKey     string
	ShardID      string
	Environment  Environment
	DatasetID    string
	Generation   string
	ContractHash string
}

type View struct {
	Namespace    Namespace
	Definition   shardv2.Resource
	Params       map[string]any
	Query        shardv2.Query
	ID           string
	URI          string
	ViewHash     string
	Select       []string
	OutputSchema shardv2.Schema
}

type Page struct {
	Records         []shardv2.Record
	NextCursor      string
	SourceUpdatedAt string
}

type Provider interface {
	Profile() shardv2.ProviderProfile
	Authorize(context.Context, View) error
	Snapshot(context.Context, View) (Page, error)
	Mutate(context.Context, View, shardv2.Command) (MutationResult, error)
}

type ChangeObserver interface {
	ObserveChanges(context.Context, View) (<-chan error, error)
}

type SubscriptionIdentity struct {
	SubscriptionID string `json:"subscriptionId"`
	Resource       string `json:"resource"`
	Epoch          string `json:"epoch"`
}

type StreamMessage struct {
	Event *shardv2.Event
	Error *Error
}

type Subscription struct {
	Identity SubscriptionIdentity
	Events   <-chan StreamMessage
	Close    func()
}

type Service interface {
	Hello(context.Context, Target) (map[string]any, error)
	Describe(context.Context, Target, ResourceRequest) (Descriptor, error)
	Read(context.Context, Target, ResourceRequest) (Snapshot, error)
	Mutate(context.Context, Target, Mutation) (MutationResult, error)
	Subscribe(context.Context, Target, ResourceRequest) (Subscription, error)
}

type Options struct {
	RefreshInterval              time.Duration
	MaxSubscriptionsPerPrincipal int
	RequestsPerSecond            int
	RequestBurst                 int
}

// Access is the composition-owned security boundary used by the resource
// application service. Implementations derive identity from context and gate
// artifact ownership; transports cannot supply these values in request JSON.
type Principal struct {
	UserID    string
	ActorType string
	ActorID   string
	Scopes    []string
}

type Access interface {
	Principal(context.Context) (Principal, error)
	RequireRead(context.Context) error
	CanWrite(context.Context) bool
	RequireApp(context.Context, string) error
	Forbidden() error
	ErrorCode(error) string
}

// Resource-prefixed aliases preserve the vocabulary of the existing service
// facade and make moving implementation code into this package mechanical.
type ResourceTarget = Target
type ResourceDescriptor = Descriptor
type ResourceSnapshot = Snapshot
type ResourceMutation = Mutation
type ResourceTombstone = Tombstone
type ResourceMutationResult = MutationResult
type ResourceError = Error
type ResourceReleaseReader = ReleaseReader
type ResourceNamespace = Namespace
type ResourceView = View
type ResourcePage = Page
type ResourceProvider = Provider
type ResourceChangeObserver = ChangeObserver
type ResourceSubscriptionIdentity = SubscriptionIdentity
type ResourceStreamMessage = StreamMessage
type ResourceSubscription = Subscription
type ShardResourceService = Service
type ResourceServiceOptions = Options

type ArchiveManifest struct {
	SHA256   string `json:"sha256"`
	Records  int    `json:"records"`
	Receipts int    `json:"receipts"`
}

// ArchiveStore is an internal recovery port, never an authored bridge or MCP
// operation. Restore implementations must reject non-empty namespaces.
type ArchiveStore interface {
	ExportResourceData(context.Context, string, Environment, io.Writer) (ArchiveManifest, error)
	RestoreResourceData(context.Context, string, Environment, io.Reader, shardv2.Registry) (ArchiveManifest, error)
}
