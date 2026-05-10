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
	SourceID       string `json:"source_id"`
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

// SourceItemPayload is the global provider-cache enrichment envelope.
// It is intentionally tenant-free: tenant matching starts after this payload is enriched.
type SourceItemPayload struct {
	SourceItemID     string         `json:"source_item_id"`
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
	SourceItemID   string `json:"source_item_id"`
	CorrelationID  string `json:"correlation_id"`
	SourceRevision int64  `json:"source_revision"`
}
