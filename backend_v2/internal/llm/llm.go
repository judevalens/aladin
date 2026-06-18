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

// ClaimEntity is a resolved entity offered as grounding context for claim extraction.
type ClaimEntity struct {
	ID   string
	Name string
}

// ClaimExtractionInput asks the model to lift CONTESTABLE, entity-grounded claims out of
// a record's raw enrichment (C0). Plain facts are rejected; only propositions something
// could support or contradict become claims.
type ClaimExtractionInput struct {
	Summary   string
	KeyClaims []string
	Entities  []ClaimEntity
}

// ExtractedClaim is one candidate claim. Polarity is assert|deny|neutral; SubjectNames
// are names of the provided entities the claim is about (matched back to ids by the caller).
type ExtractedClaim struct {
	Text         string   `json:"text"`
	Polarity     string   `json:"polarity"`
	Contestable  bool     `json:"contestable"`
	SubjectNames []string `json:"subjects"`
}

type ClaimExtractor interface {
	ExtractClaims(ctx context.Context, input ClaimExtractionInput) ([]ExtractedClaim, error)
}

// ClaimAdjudicationInput asks how a new claim relates to a candidate claim (C1) — the
// polarity-aware twist that powers the contradiction surface.
type ClaimAdjudicationInput struct {
	A         string // the new claim
	B         string // the candidate claim
	Subjects  []string
}

// ClaimRelation: "same" (paraphrase, same stance) | "negation" (same proposition,
// opposite stance) | "related" (distinct but linked) | "unrelated". When "related",
// EdgeType is supports|contradicts|qualifies.
type ClaimRelation struct {
	Relation   string  `json:"relation"`
	EdgeType   string  `json:"edge_type"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type ClaimAdjudicator interface {
	JudgeClaims(ctx context.Context, input ClaimAdjudicationInput) (*ClaimRelation, error)
}
