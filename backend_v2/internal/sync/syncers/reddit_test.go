package syncers

import (
	"context"
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

type fakeSeenStore struct {
	known map[string]bool
}

type fakeRedditClient struct {
	resp *redditListingResponse
	err  error
}

type redditFixturePost struct {
	id         string
	title      string
	selftext   string
	permalink  string
	score      int
	createdUTC float64
}

func (f fakeRedditClient) FetchListing(ctx context.Context, state redditCycleState) (*redditListingResponse, error) {
	return f.resp, f.err
}

func redditResponse(children ...redditFixturePost) *redditListingResponse {
	resp := &redditListingResponse{}
	for _, child := range children {
		var item struct {
			Data struct {
				ID         string  `json:"id"`
				Title      string  `json:"title"`
				Selftext   string  `json:"selftext"`
				Permalink  string  `json:"permalink"`
				Score      int     `json:"score"`
				CreatedUTC float64 `json:"created_utc"`
			} `json:"data"`
		}
		item.Data.ID = child.id
		item.Data.Title = child.title
		item.Data.Selftext = child.selftext
		item.Data.Permalink = child.permalink
		item.Data.Score = child.score
		item.Data.CreatedUTC = child.createdUTC
		resp.Data.Children = append(resp.Data.Children, item)
	}
	return resp
}

func (f fakeSeenStore) Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(externalIDs))
	for _, id := range externalIDs {
		if f.known[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (f fakeSeenStore) MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error {
	return nil
}

func TestRedditBuildJobBootstrapForNewSource(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.Source{
		ID:   "source-1",
		KgID: "kg-1",
		Name: "golang",
		Type: "reddit",
		Config: map[string]any{
			"subreddit": "golang",
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload.Payload["bootstrap"]; ok {
		t.Fatal("bootstrap should not be present")
	}
	if got := payload.Payload["next_last_seen_id"]; got != "" {
		t.Fatalf("next_last_seen_id = %v, want empty", got)
	}
}

func TestRedditBuildJobCarriesLastSeenBoundary(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.Source{
		ID:   "source-1",
		KgID: "kg-1",
		Name: "golang",
		Type: "reddit",
		Config: map[string]any{
			"subreddit":             "golang",
			"last_seen_id":          "seen-123",
			"last_seen_created_utc": float64(1234),
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload.Payload["stop_before_id"]; ok {
		t.Fatal("stop_before_id should not be present")
	}
	if _, ok := payload.Payload["bootstrap"]; ok {
		t.Fatal("bootstrap should not be present")
	}
}

func TestRedditExecuteStopsAtSeenID(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncerWithClient(fakeRedditClient{resp: func() *redditListingResponse {
		resp := redditResponse(
			redditFixturePost{"new-3", "newest", "", "/r/golang/comments/new3/x", 10, 300},
			redditFixturePost{"new-2", "newer", "", "/r/golang/comments/new2/x", 9, 200},
			redditFixturePost{"seen-1", "seen", "", "/r/golang/comments/seen1/x", 8, 100},
		)
		resp.Data.After = "t3_next"
		return resp
	}()}, fakeSeenStore{known: map[string]bool{"t3_seen-1": true}})

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "reddit",
		Payload: map[string]any{
			"subreddit":                  "golang",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.HasMore {
		t.Fatal("HasMore = true, want false after hitting boundary")
	}
	if result.CompletionReason != sync.CompletionReasonSeenOverlap {
		t.Fatalf("CompletionReason = %q, want %q", result.CompletionReason, sync.CompletionReasonSeenOverlap)
	}
	if len(result.Records) != 2 {
		t.Fatalf("record count = %d, want 2", len(result.Records))
	}
	if got := result.SourceUpdates["last_seen_id"]; got != "t3_new-3" {
		t.Fatalf("last_seen_id = %v, want t3_new-3", got)
	}
	if got := floatFromAny(result.SourceUpdates["last_seen_created_utc"]); got != 300 {
		t.Fatalf("last_seen_created_utc = %v, want 300", got)
	}
	if got := result.HeadBoundary["id"]; got != "t3_new-3" {
		t.Fatalf("head_boundary.id = %v, want t3_new-3", got)
	}
	if got := floatFromAny(result.HeadBoundary["created_utc"]); got != 300 {
		t.Fatalf("head_boundary.created_utc = %v, want 300", got)
	}
}

func TestRedditExecuteCarriesNextBoundaryAcrossPagination(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncerWithClient(fakeRedditClient{resp: func() *redditListingResponse {
		resp := redditResponse(
			redditFixturePost{"new-3", "newest", "", "/r/golang/comments/new3/x", 10, 300},
			redditFixturePost{"new-2", "newer", "", "/r/golang/comments/new2/x", 9, 200},
		)
		resp.Data.After = "t3_next"
		return resp
	}()}, sync.NewNoopSeenStore())

	job := &db.SyncJob{
		CorrelationID: "corr-1",
		SourceID:      "source-1",
		SourceType:    "reddit",
		Priority:      5,
		MaxAttempts:   3,
		Payload: map[string]any{
			"subreddit":                  "golang",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.HasMore {
		t.Fatal("HasMore = false, want true for pagination follow-up")
	}
	if result.CompletionReason != "" {
		t.Fatalf("CompletionReason = %q, want empty for continued pagination", result.CompletionReason)
	}
	if got := result.CursorUpdates["after"]; got != "t3_next" {
		t.Fatalf("after = %v, want t3_next", got)
	}
	if got := result.SourceUpdates["last_seen_id"]; got != "t3_new-3" {
		t.Fatalf("last_seen_id = %v, want t3_new-3", got)
	}
	if got := result.HeadBoundary["id"]; got != "t3_new-3" {
		t.Fatalf("head_boundary.id = %v, want t3_new-3", got)
	}
}

func TestRedditExecuteDoesNotAdvanceHighWaterMarkWhenFirstPostIsSeen(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(fakeSeenStore{known: map[string]bool{"t3_seen-1": true}})
	s.client = fakeRedditClient{resp: func() *redditListingResponse {
		resp := redditResponse(
			struct {
				id         string
				title      string
				selftext   string
				permalink  string
				score      int
				createdUTC float64
			}{"seen-1", "seen", "", "/r/golang/comments/seen1/x", 8, 100},
		)
		resp.Data.After = "t3_next"
		return resp
	}()}

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "reddit",
		Payload: map[string]any{
			"subreddit":                  "golang",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Records) != 0 {
		t.Fatalf("record count = %d, want 0", len(result.Records))
	}
	if result.CompletionReason != sync.CompletionReasonSeenOverlap {
		t.Fatalf("CompletionReason = %q, want %q", result.CompletionReason, sync.CompletionReasonSeenOverlap)
	}
	if _, ok := result.SourceUpdates["last_seen_id"]; ok {
		t.Fatal("last_seen_id should not advance when boundary is first post")
	}
	if len(result.HeadBoundary) != 0 {
		t.Fatalf("head_boundary = %v, want empty", result.HeadBoundary)
	}
}

func TestRedditExecuteEmitsCanonicalIDsAndProvenance(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncerWithClient(fakeRedditClient{resp: redditResponse(
		redditFixturePost{"abc123", "Pairs trading", "Cointegration notes", "/r/algotrading/comments/abc123/pairs", 17, 420},
	)}, sync.NewNoopSeenStore())

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "reddit",
		Payload: map[string]any{
			"subreddit":                  "algotrading",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(result.Records))
	}

	record := result.Records[0]
	if record.ExternalID != "t3_abc123" {
		t.Fatalf("ExternalID = %q, want t3_abc123", record.ExternalID)
	}
	if record.SourceURL != "https://www.reddit.com/r/algotrading/comments/abc123/pairs" {
		t.Fatalf("SourceURL = %q, want reddit permalink", record.SourceURL)
	}
	if record.Content != "Pairs trading\n\nCointegration notes" {
		t.Fatalf("Content = %q, want title plus selftext", record.Content)
	}
	if got := record.Metadata["platform"]; got != "reddit" {
		t.Fatalf("metadata.platform = %v, want reddit", got)
	}
	if got := record.Metadata["reddit_id"]; got != "t3_abc123" {
		t.Fatalf("metadata.reddit_id = %v, want t3_abc123", got)
	}
	if got := record.Metadata["subreddit"]; got != "algotrading" {
		t.Fatalf("metadata.subreddit = %v, want algotrading", got)
	}
	if got := record.Metadata["permalink"]; got != "/r/algotrading/comments/abc123/pairs" {
		t.Fatalf("metadata.permalink = %v, want permalink", got)
	}
	if got := record.Metadata["score"]; got != 17 {
		t.Fatalf("metadata.score = %v, want 17", got)
	}
	if got := record.Metadata["created_utc"]; got != float64(420) {
		t.Fatalf("metadata.created_utc = %v, want 420", got)
	}
}
