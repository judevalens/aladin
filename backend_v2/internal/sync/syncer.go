package sync

import (
	"context"

	"aladin/backend_v2/internal/db"
)

const (
	CompletionReasonExhausted   = "exhausted"
	CompletionReasonSeenOverlap = "seen_overlap"
	CompletionReasonCold        = "cold"
)

// RawRecord is returned by a syncer and normalized into source_items.
type RawRecord struct {
	ExternalID        string
	ParentExternalID  string
	SourceRevision    int64
	Type              string
	Label             string
	Content           string
	EnrichmentContent string
	SourceURL         string
	Metadata          map[string]any
}

// Result is what a syncer returns after executing a job.
type Result struct {
	Records               []*RawRecord
	HasMore               bool            // more pages remain — scheduler will re-claim and continue
	CompletionReason      string          // terminal reason when HasMore=false
	ProviderStreamUpdates map[string]any  // merged into provider stream config
	CursorUpdates         map[string]any  // merged into cycle cursor for follow-up work
	HeadBoundary          map[string]any  // provider-specific newest claimed boundary for the cycle
	NewCycles             []*db.SyncCycle // follow-up cycles created after records are accepted
}

// Syncer is implemented once per provider type.
type Syncer interface {
	Provider() string
	HeadQueue() string // queue for all dispatch (initial + resumed pages)
	// BuildJob constructs the next sync job for a provider stream turn.
	// The cycle carries pagination state for queued fetch_page work.
	BuildJob(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error)
	Execute(ctx context.Context, job *db.SyncJob) (*Result, error)
}
