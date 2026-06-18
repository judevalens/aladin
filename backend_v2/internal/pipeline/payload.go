package pipeline

import "aladin/backend_v2/internal/search"

// RecordPayload is the accumulating envelope passed between pipeline stages.
// Each stage reads what it needs and appends its result before forwarding.
// The full payload is written to PG once when the pipeline completes.
type RecordPayload struct {
	// Identity
	RecordID       string `json:"record_id"`
	CorrelationID  string `json:"correlation_id"`
	KgID           string `json:"kg_id"`
	ExternalID     string `json:"external_id"`
	SourceRevision int64  `json:"source_revision,omitempty"`

	// Raw content — set at ingest, never modified
	Type              string         `json:"type"`
	Label             string         `json:"label"`
	Content           string         `json:"content"`
	EnrichmentContent string         `json:"enrichment_content,omitempty"`
	SourceURL         string         `json:"source_url"`
	Metadata          map[string]any `json:"metadata,omitempty"`

	// First pass results
	Summary               string   `json:"summary,omitempty"`
	Entities              []string `json:"entities,omitempty"`
	Topics                []string `json:"topics,omitempty"`
	KeyClaims             []string `json:"key_claims,omitempty"`
	LowConfidenceEntities []string `json:"low_confidence_entities,omitempty"`

	// Search results
	SearchResolved map[string][]search.SearchResult `json:"search_resolved,omitempty"`

	// Embedding
	Embedding []float32 `json:"embedding,omitempty"`
}

// GlobalRecordPayload is the global canonical-record enrichment envelope.
// It is intentionally tenant-free: tenant matching starts after this payload is enriched.
type GlobalRecordPayload struct {
	RecordID         string         `json:"record_id"`
	CorrelationID    string         `json:"correlation_id"`
	ProviderStreamID string         `json:"provider_stream_id"`
	Provider         string         `json:"provider"`
	ExternalID       string         `json:"external_id"`
	SourceRevision   int64          `json:"source_revision"`
	Type             string         `json:"type"`
	Title            string         `json:"title"`
	ContentExcerpt   string         `json:"content_excerpt"`
	ContextExcerpt   string         `json:"context_excerpt,omitempty"`
	SourceURL        string         `json:"source_url"`
	Metadata         map[string]any `json:"metadata,omitempty"`

	Summary               string   `json:"summary,omitempty"`
	Entities              []string `json:"entities,omitempty"`
	Topics                []string `json:"topics,omitempty"`
	KeyClaims             []string `json:"key_claims,omitempty"`
	LowConfidenceEntities []string `json:"low_confidence_entities,omitempty"`
}

type TenantMatchPayload struct {
	RecordID        string           `json:"record_id"`
	CorrelationID   string           `json:"correlation_id"`
	SourceRevision  int64            `json:"source_revision"`
	InsightTriggers []InsightTrigger `json:"insight_triggers,omitempty"`
}

// ResolveEntitiesPayload drives the entity-resolution stage. It carries only what the
// worker needs to load the record and apply the revision guard; the worker reads the
// enrichment entities off the record itself.
type ResolveEntitiesPayload struct {
	RecordID       string `json:"record_id"`
	CorrelationID  string `json:"correlation_id"`
	SourceRevision int64  `json:"source_revision"`
}

// ResolveClaimsPayload drives the claim-extraction stage (chained after entity
// resolution so claims can ground on the record's resolved entities). Same shape as
// ResolveEntitiesPayload, so the entity stage's payload forwards directly.
type ResolveClaimsPayload struct {
	RecordID       string `json:"record_id"`
	CorrelationID  string `json:"correlation_id"`
	SourceRevision int64  `json:"source_revision"`
}

type InsightTrigger struct {
	KgID            string   `json:"kg_id"`
	RecordID        string   `json:"record_id"`
	SourceRevision  int64    `json:"source_revision"`
	CorrelationID   string   `json:"correlation_id"`
	SubscriptionIDs []string `json:"subscription_ids,omitempty"`
	GeneratorKeys   []string `json:"generator_keys,omitempty"`
}
