package db

import (
	"context"
	"time"
)

type RecordRepository interface {
	// SaveComplete writes a fully-processed record to PG in one shot.
	SaveComplete(ctx context.Context, a *CompletedRecord) error
}

type ProviderStreamRepository interface {
	GetByID(ctx context.Context, id string) (*ProviderStream, error)
	Ensure(ctx context.Context, stream *ProviderStream) (*ProviderStream, error)
	ClaimBatch(ctx context.Context, limit int) ([]*ProviderStream, error)
	MarkSyncStarted(ctx context.Context, id string) error
	MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error
	MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error
	MarkSyncFailed(ctx context.Context, id string) error
	Release(ctx context.Context, id string) error
}

type SyncCycleRepository interface {
	ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*SyncCycle, error)
	// Create inserts a new cycle. Caller provides ID and provider stream linkage.
	Create(ctx context.Context, cycle *SyncCycle) error
	MarkRunning(ctx context.Context, id string) error
	UpdateProgress(ctx context.Context, id string, cursor map[string]any, headBoundary map[string]any, lastHydratedAt *time.Time) error
	MarkActive(ctx context.Context, id string) error
	Complete(ctx context.Context, id string, headBoundary map[string]any, completionReason string) error
}

type InsightRepository interface {
	ExistsRecent(ctx context.Context, kgID, insightType, key, title string) (bool, error)
	Store(ctx context.Context, kgID string, insight *Insight) error
}

type KnowledgeGraphRepository interface {
	GetIDsWithEnrichedRecords(ctx context.Context) ([]string, error)
}

type SourceItemRepository interface {
	Upsert(ctx context.Context, item *SourceItem) (*SourceItemUpsertResult, error)
	Get(ctx context.Context, id string) (*SourceItem, error)
}

type SourceItemEnrichmentRepository interface {
	Save(ctx context.Context, enrichment *SourceItemEnrichment) error
	Get(ctx context.Context, sourceItemID string, sourceRevision int64) (*SourceItemEnrichment, error)
}

type SourceSubscriptionRepository interface {
	ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*SourceSubscription, error)
	Ensure(ctx context.Context, sub *SourceSubscription) (*SourceSubscription, error)
}

type TenantItemMatchRepository interface {
	Save(ctx context.Context, match *TenantItemMatch) error
}
