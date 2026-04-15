package pipeline

import "aladin/backend_v2/internal/search"

// ArtifactPayload is the accumulating envelope passed between pipeline stages.
// Each stage reads what it needs and appends its result before forwarding.
// The full payload is written to PG once when the pipeline completes.
type ArtifactPayload struct {
	// Identity
	ArtifactID     string `json:"artifact_id"`
	CorrelationID  string `json:"correlation_id"`
	KgID           string `json:"kg_id"`
	SourceID       string `json:"source_id"`
	ExternalID     string `json:"external_id"`

	// Raw content — set at ingest, never modified
	Type      string         `json:"type"`
	Label     string         `json:"label"`
	Content   string         `json:"content"`
	SourceURL string         `json:"source_url"`
	Metadata  map[string]any `json:"metadata,omitempty"`

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
