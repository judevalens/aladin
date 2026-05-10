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

// Enricher runs the first-pass LLM analysis on a raw record.
type Enricher interface {
	Enrich(ctx context.Context, content, recordType string) (*EnrichResult, error)
}

// Embedder generates a dense vector for a piece of text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type RelevanceInput struct {
	SubscriptionName string
	Policy           map[string]any
	ItemTitle        string
	ItemSummary      string
	ItemEntities     []string
	ItemTopics       []string
}

type RelevanceResult struct {
	Relevant   bool    `json:"relevant"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type RelevanceJudge interface {
	JudgeRelevance(ctx context.Context, input RelevanceInput) (*RelevanceResult, error)
}
