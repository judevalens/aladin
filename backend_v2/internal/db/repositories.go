package db

import "context"

type ArtifactRepository interface {
	// SaveComplete writes a fully-processed artifact to PG in one shot.
	SaveComplete(ctx context.Context, a *CompletedArtifact) error
	// ExistsExternal returns which externalIDs already exist for a given source.
	// Used by syncers to stop pagination when they hit already-known content.
	ExistsExternal(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error)
}

type SourceRepository interface {
	GetByID(ctx context.Context, id string) (*Source, error)
	// ClaimBatch atomically claims up to limit due sources, marking them queued.
	// Returns raw sources — callers decide how to build jobs from them.
	// Safe to call from N concurrent schedulers via FOR UPDATE SKIP LOCKED.
	ClaimBatch(ctx context.Context, limit int) ([]*Source, error)
	// MarkSyncStarted transitions a source to 'syncing' and stamps last_sync_started_at.
	// Called by the sync worker when it begins executing a job.
	MarkSyncStarted(ctx context.Context, id string) error
	// MarkSynced resets a source to idle and stamps last_synced_at = now.
	MarkSynced(ctx context.Context, id string) error
	// MarkSyncFailed resets a source to idle so it can be retried next tick.
	MarkSyncFailed(ctx context.Context, id string) error
}

type SyncJobRepository interface {
	Enqueue(ctx context.Context, job *SyncJob) error
	DequeueOne(ctx context.Context, sourceType string) (*SyncJob, error)
	MarkDead(ctx context.Context, id, errMsg string) error
	MarkPendingWithBackoff(ctx context.Context, id, errMsg string, attempts, backoffSeconds int) error
	Delete(ctx context.Context, id string) error
}

type SnapshotRepository interface {
	Create(ctx context.Context, sourceID string) (*Snapshot, error)
	Complete(ctx context.Context, id string) (*Snapshot, error)
	HasActive(ctx context.Context, sourceID string) (bool, error)
}

type InsightRepository interface {
	ExistsRecent(ctx context.Context, kgID, insightType, key, title string) (bool, error)
	Store(ctx context.Context, kgID string, insight *Insight) error
}

type KnowledgeGraphRepository interface {
	GetIDsWithEnrichedArtifacts(ctx context.Context) ([]string, error)
}
