package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type RedditCycleState = redditCycleState
type RedditListingResponse = redditListingResponse

type RedditClient interface {
	FetchListing(ctx context.Context, state redditCycleState) (*redditListingResponse, error)
}

type redditHTTPClient struct {
	client *http.Client
}

func newRedditHTTPClient() RedditClient {
	return &redditHTTPClient{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *redditHTTPClient) FetchListing(ctx context.Context, state redditCycleState) (*redditListingResponse, error) {
	url := fmt.Sprintf("%s/r/%s/new.json?limit=%d&after=%s", redditAPI, state.Subreddit, redditLimit, state.After)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit status %d", resp.StatusCode)
	}

	var body redditListingResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("reddit decode: %w", err)
	}
	return &body, nil
}
