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

// EntityAdjudicationInput asks whether two entity surface forms denote the same
// real-world entity. Used by entity resolution (R2) to adjudicate the ambiguous
// fuzzy band — the cases a string normalizer can't settle (acronyms, rebrands,
// same-name/different-thing).
type EntityAdjudicationInput struct {
	Kind        string
	A           string   // the new mention's surface form
	B           string   // the candidate entity's canonical name
	BAliases    []string // the candidate's known surface forms
	ContextHint string   // optional surrounding context for A (co-mentions, summary)
}

// EntityVerdict is the adjudicator's decision. Verdict is "same" | "different" |
// "uncertain"; "different" is treated as negative evidence (the pair won't be proposed).
type EntityVerdict struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type EntityAdjudicator interface {
	JudgeSameEntity(ctx context.Context, input EntityAdjudicationInput) (*EntityVerdict, error)
}

// DiscourseMember is one document in a bridge's connected set (input to the discourse pass).
type DiscourseMember struct {
	ID      string // record/artifact id — the grounding handle
	Kind    string // record | artifact (artifact = the user's own writing)
	Summary string
}

// DiscoursePosition is one member's stance toward the bridge entity, grounded to its id.
type DiscoursePosition struct {
	MemberID string `json:"member_id"`
	Stance   string `json:"stance"` // supportive | critical | neutral | mixed
	Claim    string `json:"claim"`
}

// DiscourseResult is the discourse map for one bridge entity: each member's stance plus an
// overall reading (consensus | contradiction | mixed | emerging).
type DiscourseResult struct {
	Headline   string              `json:"headline"`
	Overall    string              `json:"overall"`
	Positions  []DiscoursePosition `json:"positions"`
	Confidence float64             `json:"confidence"`
}

// DiscourseJudge runs the stance/discourse pass over a bridge's connected members.
type DiscourseJudge interface {
	JudgeDiscourse(ctx context.Context, entity string, members []DiscourseMember) (*DiscourseResult, error)
}
