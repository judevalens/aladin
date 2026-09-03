package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ErrHTTP is returned for non-200 responses so callers can classify by status code.
type ErrHTTP struct {
	StatusCode int
}

func (e *ErrHTTP) Error() string {
	return fmt.Sprintf("tavily status %d", e.StatusCode)
}

func (e *ErrHTTP) IsRateLimit() bool { return e.StatusCode == http.StatusTooManyRequests }
func (e *ErrHTTP) IsPermanent() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
}
func (e *ErrHTTP) IsTransient() bool { return !e.IsRateLimit() && !e.IsPermanent() }

const tavilyEndpoint = "https://api.tavily.com/search"

type TavilyClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewTavilyClient(apiKey string) *TavilyClient {
	return &TavilyClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (c *TavilyClient) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	body, _ := json.Marshal(map[string]any{
		"api_key":      c.apiKey,
		"query":        query,
		"search_depth": "basic",
		"max_results":  maxResults,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilyEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tavily request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrHTTP{StatusCode: resp.StatusCode}
	}

	var result struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tavily decode: %w", err)
	}

	return result.Results, nil
}
