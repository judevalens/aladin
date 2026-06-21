package db

import (
	"context"
	"time"
)

type RecordRepository interface {
	// SaveComplete writes a fully-processed record to PG in one shot.
	SaveComplete(ctx context.Context, a *CompletedRecord) error
	UpsertCanonical(ctx context.Context, record *Record) (*RecordUpsertResult, error)
	Get(ctx context.Context, id string) (*Record, error)
	SaveEnrichment(ctx context.Context, enrichment *RecordEnrichment) (bool, error)
	// SaveEmbedding writes a record's vector and marks it complete (the terminal stage of
	// the record→knowledge chain). Revision-guarded; returns false if superseded.
	SaveEmbedding(ctx context.Context, recordID string, sourceRevision int64, vec []float32) (bool, error)
	// ListStuck returns non-terminal records untouched for olderThanSecs — records the
	// reaper re-drives (e.g. stranded by a crash between upsert and enqueue).
	ListStuck(ctx context.Context, olderThanSecs, limit int) ([]*Record, error)
	// MarkFailed moves a record to the terminal 'failed' status (a permanent error). No-op
	// if already terminal.
	MarkFailed(ctx context.Context, recordID, reason string) error
	// ResetForRetry moves a 'failed' record back to 'captured' so it re-enters the pipeline
	// (the reaper picks it up). Returns false if the record wasn't failed.
	ResetForRetry(ctx context.Context, recordID string) (bool, error)
}

type ProviderStreamRepository interface {
	GetByID(ctx context.Context, id string) (*ProviderStream, error)
	Ensure(ctx context.Context, stream *ProviderStream) (*ProviderStream, error)
	ClaimBatch(ctx context.Context, limit int) ([]*ProviderStream, error)
	MarkSyncStarted(ctx context.Context, id string) error
	Heartbeat(ctx context.Context, id string) error
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
	MarkFailed(ctx context.Context, id string, completionReason string) error
	Complete(ctx context.Context, id string, headBoundary map[string]any, completionReason string) error
}

type InsightRepository interface {
	ExistsRecent(ctx context.Context, kgID, insightType, key, title string) (bool, error)
	Store(ctx context.Context, kgID string, insight *Insight) error
}

type KnowledgeGraphRepository interface {
	GetIDsWithEnrichedRecords(ctx context.Context) ([]string, error)
}

type SourceSubscriptionRepository interface {
	ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*SourceSubscription, error)
	Ensure(ctx context.Context, sub *SourceSubscription) (*SourceSubscription, error)
}

type TenantItemMatchRepository interface {
	Save(ctx context.Context, match *TenantItemMatch) error
}

// EntityRepository is the shared (Tier 0) entity registry used by entity resolution.
type EntityRepository interface {
	FindSharedByKey(ctx context.Context, kind, normalizedKey string) ([]EntityCandidate, error)
	CreateSharedEntity(ctx context.Context, p CreateEntityParams) (string, error)
	AddAlias(ctx context.Context, p AliasParams) error
	AddMention(ctx context.Context, p MentionParams) error

	// R1: fuzzy near-match detection + proposed-merge curation.
	FindSharedCandidates(ctx context.Context, kind, normalizedKey string, minSim float64, limit int) ([]ScoredCandidate, error)
	ProposeMerge(ctx context.Context, p ProposeMergeParams) (bool, error)
	RecordDistinct(ctx context.Context, fromEntityID, intoEntityID, method, reason string) error
	ListProposedMerges(ctx context.Context, limit int) ([]ProposedMerge, error)
	RejectMerge(ctx context.Context, mergeID string) error
	AcceptMerge(ctx context.Context, mergeID string) error
	RevertMerge(ctx context.Context, mergeID string) error

	// R3: tenant tier (Tier 1 overlays + cross-tier bind + local-override read).
	FindTenantByKey(ctx context.Context, ownerUserID, kind, normalizedKey string) ([]EntityCandidate, error)
	CreateTenantEntity(ctx context.Context, p CreateTenantEntityParams) (string, error)
	ResolveCanonicalRoot(ctx context.Context, entityID string) (string, error)

	// R2 embeddings: context-embedding storage + vector candidate signal.
	SetEntityEmbedding(ctx context.Context, entityID string, vec []float32) error
	GetEntityEmbedding(ctx context.Context, entityID string) ([]float32, bool, error)
	FindSharedCandidatesByVector(ctx context.Context, kind, excludeKey string, vec []float32, minCosine float64, limit int) ([]ScoredCandidate, error)

	// Claim layer grounding: the entities a record mentions.
	EntitiesForRecord(ctx context.Context, recordID string) ([]EntityRef, error)
	// Authored grounding (P3): the entities a page is tagged with / @mentions —
	// the resolved set that grounds authored claim extraction.
	EntitiesForArtifact(ctx context.Context, artifactID string) ([]EntityRef, error)
}

// DiscourseRepository sources bridges (shared entities mentioned by >=2 docs) + members
// from the resolved entity layer (entity_mentions), keyed on entity_id, and stores the
// discourse maps. The rework of buck's Neo4j/canonical-text engine onto the entity layer.
type DiscourseRepository interface {
	CandidateBridges(ctx context.Context, minDegree, limit int) ([]Bridge, error)
	BridgeMembers(ctx context.Context, entityID string, limit int) ([]BridgeMember, error)
	GetDiscourseLedger(ctx context.Context, entityID string) (*EntityDiscourse, error)
	MarkAnalyzed(ctx context.Context, entityID string, degree int) (int, error)
	StoreDiscourse(ctx context.Context, d *DiscourseInsight) error
}

// ClaimRepository is the claim registry (C0): contestable propositions grounded in
// entities, with the evidence (claim_mentions) of which sources assert/deny them.
type ClaimRepository interface {
	FindClaimByText(ctx context.Context, scope, ownerUserID, canonicalText string) (string, bool, error)
	CreateClaim(ctx context.Context, p CreateClaimParams) (string, error)
	AddClaimSubject(ctx context.Context, claimID, entityID string) error
	AddClaimMention(ctx context.Context, p ClaimMentionParams) error

	// C1: polarity-aware resolution.
	SetClaimEmbedding(ctx context.Context, claimID string, vec []float32) error
	FindClaimCandidates(ctx context.Context, scope, ownerUserID string, subjectEntityIDs []string, vec []float32, minCosine float64, limit int) ([]ScoredClaim, error)
	AddClaimEdge(ctx context.Context, p ClaimEdgeParams) (bool, error)

	// C4: the contradiction surface for a thesis claim.
	ContradictionSurface(ctx context.Context, thesisClaimID string, minCosine float64) (ClaimStanceCounts, error)
}
