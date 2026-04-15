package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type BlueskyCycleState = blueskyCycleState
type BlueskySearchPostsResponse = blueskySearchPostsResponse

type BlueskyClient interface {
	SearchPosts(ctx context.Context, state blueskyCycleState) (*blueskySearchPostsResponse, error)
}

type blueskyHTTPClient struct {
	client *http.Client
}

func newBlueskyHTTPClient() BlueskyClient {
	return &blueskyHTTPClient{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *blueskyHTTPClient) SearchPosts(ctx context.Context, state blueskyCycleState) (*blueskySearchPostsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blueskyAPI+"/app.bsky.feed.searchPosts", nil)
	if err != nil {
		return nil, fmt.Errorf("bluesky request: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", state.Query)
	q.Set("sort", "latest")
	q.Set("limit", fmt.Sprintf("%d", blueskyPageSize))
	if state.Cursor != "" {
		q.Set("cursor", state.Cursor)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bluesky fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky status %d", resp.StatusCode)
	}

	var body blueskySearchPostsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("bluesky decode: %w", err)
	}
	return &body, nil
}
