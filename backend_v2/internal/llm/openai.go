package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// firstPassSchema is the JSON Schema sent to OpenAI structured outputs.
// strict=true means the model is guaranteed to return this exact shape.
var firstPassSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary": map[string]any{
			"type":        "string",
			"description": "1-2 sentence summary of the content",
		},
		"entities": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "people, organizations, tools, or concepts explicitly mentioned",
		},
		"topics": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "2-5 high-level topic tags",
		},
		"key_claims": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "main points or arguments stated",
		},
		"low_confidence_entities": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "entities that are unknown, niche, or potentially recent",
		},
	},
	"required":             []string{"summary", "entities", "topics", "key_claims", "low_confidence_entities"},
	"additionalProperties": false,
}

var relevanceSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"relevant": map[string]any{
			"type":        "boolean",
			"description": "Whether this record is relevant to the subscription intent.",
		},
		"confidence": map[string]any{
			"type":        "number",
			"description": "Confidence from 0 to 1.",
		},
		"reason": map[string]any{
			"type":        "string",
			"description": "Short reason for the decision.",
		},
	},
	"required":             []string{"relevant", "confidence", "reason"},
	"additionalProperties": false,
}

var entityAdjudicationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"verdict": map[string]any{
			"type":        "string",
			"enum":        []string{"same", "different", "uncertain"},
			"description": "Whether A and B denote the same real-world entity.",
		},
		"confidence": map[string]any{
			"type":        "number",
			"description": "Confidence from 0 to 1.",
		},
		"reason": map[string]any{
			"type":        "string",
			"description": "Short reason for the decision.",
		},
	},
	"required":             []string{"verdict", "confidence", "reason"},
	"additionalProperties": false,
}

// OpenAIEntityAdjudicator implements EntityAdjudicator. NOTE: like
// OpenAIRelevanceJudge it is not wired into the live pipeline by default — entity
// resolution runs with a nil adjudicator (deterministic R1 behavior) until this is
// deliberately enabled and verified against a real key.
type OpenAIEntityAdjudicator struct {
	client openai.Client
}

func NewOpenAIEntityAdjudicator(apiKey string) *OpenAIEntityAdjudicator {
	return &OpenAIEntityAdjudicator{client: openai.NewClient(option.WithAPIKey(apiKey))}
}

func (j *OpenAIEntityAdjudicator) JudgeSameEntity(ctx context.Context, input EntityAdjudicationInput) (*EntityVerdict, error) {
	resp, err := j.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You decide whether two names refer to the same real-world entity. Same-name look-alikes of the same kind are often DIFFERENT entities; demand evidence before saying 'same'. Prefer 'uncertain' over a guess."),
			openai.UserMessage(fmt.Sprintf(
				"Kind: %s\nA: %s\nContext for A: %s\nB (candidate): %s\nKnown aliases of B: %v",
				input.Kind, input.A, input.ContextHint, input.B, input.BAliases,
			)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "entity_verdict",
					Strict: openai.Bool(true),
					Schema: entityAdjudicationSchema,
				},
			},
		},
		MaxTokens: openai.Int(180),
	})
	if err != nil {
		return nil, fmt.Errorf("openai entity adjudication: %w", err)
	}
	var result EntityVerdict
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse entity verdict: %w", err)
	}
	return &result, nil
}

var claimExtractionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"claims": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text":        map[string]any{"type": "string", "description": "the claim as one canonical sentence"},
					"polarity":    map[string]any{"type": "string", "enum": []string{"assert", "deny", "neutral"}, "description": "assert = stated affirmatively, deny = stated negatively"},
					"contestable": map[string]any{"type": "boolean", "description": "true ONLY if something could support or contradict it (a prediction, opinion, or evaluative/causal claim); false for plain facts"},
					"subjects":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "names of the provided entities this claim is about"},
				},
				"required":             []string{"text", "polarity", "contestable", "subjects"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"claims"},
	"additionalProperties": false,
}

// OpenAIClaimExtractor implements ClaimExtractor. Like the other judges it is wired with
// graceful degradation and its live call is unverified without a real key.
type OpenAIClaimExtractor struct {
	client openai.Client
}

func NewOpenAIClaimExtractor(apiKey string) *OpenAIClaimExtractor {
	return &OpenAIClaimExtractor{client: openai.NewClient(option.WithAPIKey(apiKey))}
}

func (e *OpenAIClaimExtractor) ExtractClaims(ctx context.Context, input ClaimExtractionInput) ([]ExtractedClaim, error) {
	names := make([]string, 0, len(input.Entities))
	for _, en := range input.Entities {
		names = append(names, en.Name)
	}
	resp, err := e.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You lift CONTESTABLE claims (predictions, opinions, evaluative/causal assertions) out of a record. Reject plain facts. Each claim must be about one of the provided entities. Be conservative — prefer fewer, sharper claims."),
			openai.UserMessage(fmt.Sprintf(
				"Summary: %s\nRaw points: %v\nKnown entities (claims must reference these by name): %v",
				input.Summary, input.KeyClaims, names,
			)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "claim_extraction",
					Strict: openai.Bool(true),
					Schema: claimExtractionSchema,
				},
			},
		},
		MaxTokens: openai.Int(500),
	})
	if err != nil {
		return nil, fmt.Errorf("openai claim extraction: %w", err)
	}
	var out struct {
		Claims []ExtractedClaim `json:"claims"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("parse claim extraction: %w", err)
	}
	return out.Claims, nil
}

// OpenAIEnricher implements Enricher using OpenAI structured outputs.
type OpenAIEnricher struct {
	client openai.Client
}

func NewOpenAIEnricher(apiKey string) *OpenAIEnricher {
	return &OpenAIEnricher{client: openai.NewClient(option.WithAPIKey(apiKey))}
}

func (e *OpenAIEnricher) Enrich(ctx context.Context, content, recordType string) (*EnrichResult, error) {
	resp, err := e.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a research intelligence assistant."),
			openai.UserMessage(fmt.Sprintf(
				"Analyze this %s and extract structured information.\n\nContent: %s",
				recordType, content,
			)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "first_pass_result",
					Strict: openai.Bool(true),
					Schema: firstPassSchema, // pass map[string]any directly — []byte gets base64-encoded
				},
			},
		},
		MaxTokens: openai.Int(500),
	})
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	var result EnrichResult
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &result, nil
}

// OpenAIEmbedder implements Embedder using OpenAI text-embedding-3-small.
type OpenAIEmbedder struct {
	client openai.Client
}

func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
	return &OpenAIEmbedder{client: openai.NewClient(option.WithAPIKey(apiKey))}
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModelTextEmbedding3Small,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: []string{text},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai embeddings: %w", err)
	}

	raw := resp.Data[0].Embedding
	vector := make([]float32, len(raw))
	for i, v := range raw {
		vector[i] = float32(v)
	}
	return vector, nil
}

type OpenAIRelevanceJudge struct {
	client openai.Client
}

func NewOpenAIRelevanceJudge(apiKey string) *OpenAIRelevanceJudge {
	return &OpenAIRelevanceJudge{client: openai.NewClient(option.WithAPIKey(apiKey))}
}

func (j *OpenAIRelevanceJudge) JudgeRelevance(ctx context.Context, input RelevanceInput) (*RelevanceResult, error) {
	policyJSON, _ := json.Marshal(input.Policy)
	resp, err := j.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You judge whether a globally enriched record is useful for a user's subscribed research stream. Be conservative."),
			openai.UserMessage(fmt.Sprintf(
				"Subscription: %s\nPolicy: %s\n\nItem title: %s\nSummary: %s\nEntities: %v\nTopics: %v",
				input.SubscriptionName,
				string(policyJSON),
				input.ItemTitle,
				input.ItemSummary,
				input.ItemEntities,
				input.ItemTopics,
			)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "relevance_result",
					Strict: openai.Bool(true),
					Schema: relevanceSchema,
				},
			},
		},
		MaxTokens: openai.Int(180),
	})
	if err != nil {
		return nil, fmt.Errorf("openai relevance: %w", err)
	}
	var result RelevanceResult
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse relevance: %w", err)
	}
	return &result, nil
}
