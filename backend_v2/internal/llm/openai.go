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
