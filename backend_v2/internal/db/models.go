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

// Source is a configured data source.
type Source struct {
	ID            string
	KgID          string
	Name          string
	Type          string
	Config        map[string]any
	SyncStatus    string     // idle | queued | syncing
	SyncInterval  int        // seconds between syncs
	LastPickedAt  *time.Time // scheduler last gave this source a turn
	LastRefreshAt *time.Time // last completed refresh/sync cycle
}

// SyncCycle tracks one traversal over a source feed.
// Source scheduling decides which cycle gets the next turn.
type SyncCycle struct {
	ID               string
	SourceID         string
	Kind             string         // refresh | backfill
	Status           string         // active | running | complete | closed
	Cursor           map[string]any // pagination continuation state
	HeadBoundary     map[string]any // newest claimed boundary for this cycle
	CompletionReason string
	LastPickedAt     *time.Time
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

// SyncJob is one unit of work in the sync queue.
type SyncJob struct {
	ID            string
	CorrelationID string
	CycleID       string
	SourceID      string
	KgID          string
	SnapshotID    string
	SourceType    string
	JobType       string
	Payload       map[string]any
	Priority      int
	Attempts      int
	MaxAttempts   int
	LastError     string
}

// Snapshot tracks one sync cycle for a source.
type Snapshot struct {
	ID       string
	SourceID string
	KgID     string
	Version  int
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
	ID         string
	ExternalID string
	SourceID   string
	Type       string
	Label      string
	Content    string
	SourceURL  string
	Metadata   map[string]any
	Enrichment []byte    // JSON
	Embedding  []float32 // pgvector
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
