package llm

import "context"

// EnrichResult is the structured output from the first-pass enrichment stage.
type EnrichResult struct {
	Summary               string   `json:"summary"`
	Entities              []string `json:"entities"`
	Topics                []string `json:"topics"`
	KeyClaims             []string `json:"key_claims"`
	LowConfidenceEntities []string `json:"low_confidence_entities"`
}

// Enricher runs the first-pass LLM analysis on a raw artifact.
type Enricher interface {
	Enrich(ctx context.Context, content, artifactType string) (*EnrichResult, error)
}

// Embedder generates a dense vector for a piece of text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
