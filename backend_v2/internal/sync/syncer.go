package sync

import (
	"context"

	"aladin/backend_v2/internal/db"
)

const (
	CompletionReasonExhausted   = "exhausted"
	CompletionReasonSeenOverlap = "seen_overlap"
)

// RawArtifact is returned by a syncer — gets persisted to the artifacts table.
type RawArtifact struct {
	ExternalID       string
	ParentExternalID string
	Type             string
	Label            string
	Content          string
	SourceURL        string
	Metadata         map[string]any
}

// Result is what a syncer returns after executing a job.
type Result struct {
	Artifacts        []*RawArtifact
	HasMore          bool           // more pages remain — scheduler will re-claim and continue
	CompletionReason string         // terminal reason when HasMore=false
	SourceUpdates    map[string]any // merged into source config
	CursorUpdates    map[string]any // merged into cycle cursor for follow-up fetch_page
	HeadBoundary     map[string]any // source-specific newest claimed boundary for the cycle
}

// Syncer is implemented once per source type.
type Syncer interface {
	SourceType() string
	HeadQueue() string // queue for all dispatch (initial + resumed pages)
	// BuildJob constructs the next sync job for a source turn.
	// The cycle carries pagination state for queued fetch_page work.
	BuildJob(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error)
	Execute(ctx context.Context, job *db.SyncJob) (*Result, error)
}
