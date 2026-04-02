package sync

import (
	"context"

	"aladin/backend_v2/internal/db"
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
	Artifacts  []*RawArtifact
	NextJob    *db.SyncJob // pagination follow-up
	RequeueJob *db.SyncJob // retry with backoff
}

// Syncer is implemented once per source type.
type Syncer interface {
	SourceType() string
	// BuildJob constructs the initial sync job for a source.
	// Owns job type, payload shape, retry policy, and timeout.
	// Returns an error if the source config is invalid or missing required fields.
	BuildJob(source db.Source) (*db.ScheduledJob, error)
	Execute(ctx context.Context, job *db.SyncJob) (*Result, error)
}
