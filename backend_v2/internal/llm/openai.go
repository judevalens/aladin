package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

var discourseSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"headline": map[string]any{"type": "string", "description": "one sentence capturing the discourse around the entity"},
		"overall": map[string]any{
			"type": "string", "enum": []string{"consensus", "contradiction", "mixed", "emerging"},
			"description": "consensus = sources agree; contradiction = directly oppose; mixed = varied; emerging = too sparse to tell.",
		},
		"positions": map[string]any{
			"type":        "array",
			"description": "one entry per provided member that takes a discernible position",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"member_id": map[string]any{"type": "string", "description": "the member id exactly as provided"},
					"stance":    map[string]any{"type": "string", "enum": []string{"supportive", "critical", "neutral", "mixed"}},
					"claim":     map[string]any{"type": "string", "description": "this member's position on the entity, one line"},
				},
				"required":             []string{"member_id", "stance", "claim"},
				"additionalProperties": false,
			},
		},
		"confidence": map[string]any{"type": "number"},
	},
	"required":             []string{"headline", "overall", "positions", "confidence"},
	"additionalProperties": false,
}

// OpenAIDiscourseJudge runs the discourse/stance pass. Live call unverified without a key;
// the discourse sweep degrades gracefully without it.
type OpenAIDiscourseJudge struct {
	client openai.Client
}

func NewOpenAIDiscourseJudge(apiKey string) *OpenAIDiscourseJudge {
	return &OpenAIDiscourseJudge{client: openai.NewClient(option.WithAPIKey(apiKey))}
}

func (j *OpenAIDiscourseJudge) JudgeDiscourse(ctx context.Context, entity string, members []DiscourseMember) (*DiscourseResult, error) {
	var b strings.Builder
	for _, m := range members {
		fmt.Fprintf(&b, "- [%s] (%s) %s\n", m.ID, m.Kind, m.Summary)
	}
	resp, err := j.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You analyze how multiple sources discuss one entity. Identify each source's stance toward the entity and whether the sources agree or contradict. Artifact members are the user's own writing — weigh them as the user's position. Ground every position to the member id provided. Be conservative; mark 'emerging' when evidence is thin."),
			openai.UserMessage(fmt.Sprintf("Entity: %s\n\nSources:\n%s", entity, b.String())),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "discourse_result",
					Strict: openai.Bool(true),
					Schema: discourseSchema,
				},
			},
		},
		MaxTokens: openai.Int(700),
	})
	if err != nil {
		return nil, fmt.Errorf("openai discourse: %w", err)
	}
	var result DiscourseResult
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse discourse: %w", err)
	}
	return &result, nil
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
