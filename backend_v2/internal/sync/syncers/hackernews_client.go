package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HackerNewsCycleState = hackerNewsCycleState
type HackerNewsItem = hackerNewsItem

type HackerNewsClient interface {
	FetchStoryIDs(ctx context.Context, feed string) ([]int64, error)
	FetchItem(ctx context.Context, id int64) (*hackerNewsItem, error)
}

type hackerNewsHTTPClient struct {
	client *http.Client
}

func newHackerNewsHTTPClient() HackerNewsClient {
	return &hackerNewsHTTPClient{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *hackerNewsHTTPClient) FetchStoryIDs(ctx context.Context, feed string) ([]int64, error) {
	feed = strings.TrimSpace(feed)
	if feed == "" {
		feed = "topstories"
	}
	url := fmt.Sprintf("%s/v0/%s.json", hackerNewsAPI, feed)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("hackernews request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hackernews fetch stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hackernews stories status %d", resp.StatusCode)
	}

	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("hackernews stories decode: %w", err)
	}
	return ids, nil
}

func (h *hackerNewsHTTPClient) FetchItem(ctx context.Context, id int64) (*hackerNewsItem, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", hackerNewsAPI, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("hackernews item request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hackernews item fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hackernews item status %d", resp.StatusCode)
	}

	var item hackerNewsItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("hackernews item decode: %w", err)
	}
	return &item, nil
}
