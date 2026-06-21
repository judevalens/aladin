package pipeline

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
