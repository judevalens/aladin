package db

import (
	"time"
)

// ScheduledJob is a generic unit of work returned by ClaimBatch.
// The scheduler only sees this — no domain knowledge required.
type ScheduledJob struct {
	ID       string
	Type     string
	Payload  []byte
	Priority int
	MaxRetry int
	Timeout  time.Duration
}

// Source is a configured data source.
type Source struct {
	ID           string
	KgID         string
	Name         string
	Type         string
	Config       map[string]any
	SyncStatus   string // idle | queued | syncing
	SyncInterval int    // seconds between syncs
}

// SyncJob is one unit of work in the sync queue.
type SyncJob struct {
	ID          string
	SourceID    string
	KgID        string
	SnapshotID  string
	SourceType  string
	JobType     string
	Payload     map[string]any
	Priority    int
	Attempts    int
	MaxAttempts int
	LastError   string
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
	Type        string
	Title       string
	Body        string
	Entity      string
	Topic       string
	ArtifactIDs []string
	Confidence  float64
}

// CompletedArtifact is written to PG in a single INSERT when the pipeline finishes.
type CompletedArtifact struct {
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

// EmbeddedArtifact is passed to the GraphPromoter after pipeline completion.
type EmbeddedArtifact struct {
	ID         string
	Type       string
	Label      string
	SourceURL  string
	Enrichment map[string]any
	CreatedAt  time.Time
}
