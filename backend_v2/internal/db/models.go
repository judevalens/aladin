package db

import (
	"time"
)

// ScheduledJob is a generic unit of work returned by ClaimBatch.
// The scheduler only sees this — no domain knowledge required.
type ScheduledJob struct {
	ID            string
	CorrelationID string
	Type          string
	Payload       []byte
	Priority      int
	MaxRetry      int
	Timeout       time.Duration
}

// ProviderStream is a globally reusable public provider stream.
// Product-facing user subscriptions point at this row instead of owning fetch policy.
type ProviderStream struct {
	ID            string
	Provider      string
	StreamKind    string
	StreamKey     string
	Name          string
	Config        map[string]any
	SyncStatus    string
	SyncInterval  int
	LastPickedAt  *time.Time
	LastRefreshAt *time.Time
}

// SyncCycle tracks one traversal over a provider stream.
// Provider stream scheduling decides which cycle gets the next turn.
type SyncCycle struct {
	ID               string
	ProviderStreamID string
	Kind             string         // refresh | hydration | backfill
	Status           string         // active | running | complete | closed
	Cursor           map[string]any // pagination continuation state
	HeadBoundary     map[string]any // newest claimed boundary for this cycle
	LastHydratedAt   *time.Time
	TargetKind       string
	TargetExternalID string
	CompletionReason string
	LastPickedAt     *time.Time
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

// SyncJob is one unit of work in the sync queue.
type SyncJob struct {
	ID               string
	CorrelationID    string
	CycleID          string
	ProviderStreamID string
	Provider         string
	JobType          string
	Payload          map[string]any
	Priority         int
	Attempts         int
	MaxAttempts      int
	LastError        string
}

// Insight is an AI-generated signal from the knowledge graph.
type Insight struct {
	Type       string
	Title      string
	Body       string
	Entity     string
	Topic      string
	RecordIDs  []string
	Confidence float64
}

// CompletedRecord is written to PG in a single INSERT when the pipeline finishes.
type CompletedRecord struct {
	ID             string
	ExternalID     string
	SourceRevision int64
	Type           string
	Label          string
	Content        string
	SourceURL      string
	Metadata       map[string]any
	Enrichment     []byte    // JSON
	Embedding      []float32 // pgvector
}

type SourceItem struct {
	ID                string
	ProviderStreamID  string
	Provider          string
	ExternalID        string
	OwnerUserID       *string
	Type              string
	Title             string
	CanonicalURL      string
	SourceURL         string
	ContentExcerpt    string
	ContextExcerpt    string
	SourceRevision    int64
	ProviderMetadata  map[string]any
	HydrationMetadata map[string]any
	FetchedAt         time.Time
	HydratedAt        *time.Time
}

type SourceItemUpsertResult struct {
	Item     *SourceItem
	Changed  bool
	IsStale  bool
	Inserted bool
}

type SourceItemEnrichment struct {
	SourceItemID          string
	SourceRevision        int64
	Summary               string
	Entities              []string
	Topics                []string
	KeyClaims             []string
	LowConfidenceEntities []string
	Embedding             []float32
	Status                string
}

type SourceSubscription struct {
	ID               string
	UserID           string
	KgID             string
	ProviderStreamID string
	Name             string
	Visibility       string
	Policy           map[string]any
	Status           string
}

type TenantItemMatch struct {
	SubscriptionID  string
	SourceItemID    string
	SourceRevision  int64
	MatchSource     string
	OverlapEntities []string
	RelevanceStatus string
	RelevanceScore  *float64
	RelevanceReason string
	RecordID        *string
}

// EmbeddedRecord is passed to the GraphPromoter after pipeline completion.
type EmbeddedRecord struct {
	ID         string
	Type       string
	Label      string
	SourceURL  string
	Enrichment map[string]any
	CreatedAt  time.Time
}
