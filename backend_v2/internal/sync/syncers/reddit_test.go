package syncers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

type fakeSeenStore struct {
	known map[string]bool
}

type fakeRedditClient struct {
	resp        *redditListingResponse
	err         error
	threads     map[string]*redditThreadResponse
	threadErrs  map[string]error
	threadCalls []string
}

type fakeRequestLimiter struct {
	calls int
	err   error
}

func (f *fakeRequestLimiter) Wait(ctx context.Context) error {
	f.calls++
	return f.err
}

func newTestRedditSyncer(client *fakeRedditClient, seen sync.SeenStore) (*RedditSyncer, *fakeRequestLimiter) {
	s := NewRedditSyncerWithClient(client, seen)
	limiter := &fakeRequestLimiter{}
	s.limiter = limiter
	return s, limiter
}

type redditFixturePost struct {
	id          string
	title       string
	selftext    string
	permalink   string
	score       int
	numComments int
	createdUTC  float64
}

type redditFixtureComment struct {
	id         string
	body       string
	permalink  string
	score      int
	createdUTC float64
}

func (f fakeRedditClient) FetchListing(ctx context.Context, state redditCycleState) (*redditListingResponse, error) {
	return f.resp, f.err
}

func (f *fakeRedditClient) FetchThread(ctx context.Context, subreddit string, postID string, limit int) (*redditThreadResponse, error) {
	f.threadCalls = append(f.threadCalls, redditExternalID(postID))
	if err := f.threadErrs[postID]; err != nil {
		return nil, err
	}
	if err := f.threadErrs[redditExternalID(postID)]; err != nil {
		return nil, err
	}
	if resp := f.threads[postID]; resp != nil {
		return resp, nil
	}
	if resp := f.threads[redditExternalID(postID)]; resp != nil {
		return resp, nil
	}
	return redditThreadResponseFor(redditFixturePost{
		id: postID, title: "hydrated " + postID, permalink: "/r/" + subreddit + "/comments/" + postID + "/x",
	}), nil
}

func redditResponse(children ...redditFixturePost) *redditListingResponse {
	resp := &redditListingResponse{}
	for _, child := range children {
		var item struct {
			Data struct {
				ID          string  `json:"id"`
				Title       string  `json:"title"`
				Selftext    string  `json:"selftext"`
				Permalink   string  `json:"permalink"`
				Score       int     `json:"score"`
				NumComments int     `json:"num_comments"`
				CreatedUTC  float64 `json:"created_utc"`
			} `json:"data"`
		}
		item.Data.ID = child.id
		item.Data.Title = child.title
		item.Data.Selftext = child.selftext
		item.Data.Permalink = child.permalink
		item.Data.Score = child.score
		item.Data.NumComments = child.numComments
		item.Data.CreatedUTC = child.createdUTC
		resp.Data.Children = append(resp.Data.Children, item)
	}
	return resp
}

func redditThreadResponseFor(post redditFixturePost, comments ...redditFixtureComment) *redditThreadResponse {
	resp := redditThreadResponse{}
	var postListing struct {
		Data struct {
			Children []struct {
				Kind string `json:"kind"`
				Data struct {
					ID          string  `json:"id"`
					Title       string  `json:"title"`
					Selftext    string  `json:"selftext"`
					Body        string  `json:"body"`
					Permalink   string  `json:"permalink"`
					Score       int     `json:"score"`
					NumComments int     `json:"num_comments"`
					CreatedUTC  float64 `json:"created_utc"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	postListing.Data.Children = append(postListing.Data.Children, redditThreadChild("t3", post.id, post.title, post.selftext, "", post.permalink, post.score, post.numComments, post.createdUTC))

	var commentListing struct {
		Data struct {
			Children []struct {
				Kind string `json:"kind"`
				Data struct {
					ID          string  `json:"id"`
					Title       string  `json:"title"`
					Selftext    string  `json:"selftext"`
					Body        string  `json:"body"`
					Permalink   string  `json:"permalink"`
					Score       int     `json:"score"`
					NumComments int     `json:"num_comments"`
					CreatedUTC  float64 `json:"created_utc"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	for _, comment := range comments {
		commentListing.Data.Children = append(commentListing.Data.Children, redditThreadChild("t1", comment.id, "", "", comment.body, comment.permalink, comment.score, 0, comment.createdUTC))
	}
	resp = append(resp, postListing, commentListing)
	return &resp
}

func redditThreadChild(kind string, id string, title string, selftext string, body string, permalink string, score int, numComments int, createdUTC float64) struct {
	Kind string `json:"kind"`
	Data struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Selftext    string  `json:"selftext"`
		Body        string  `json:"body"`
		Permalink   string  `json:"permalink"`
		Score       int     `json:"score"`
		NumComments int     `json:"num_comments"`
		CreatedUTC  float64 `json:"created_utc"`
	} `json:"data"`
} {
	var child struct {
		Kind string `json:"kind"`
		Data struct {
			ID          string  `json:"id"`
			Title       string  `json:"title"`
			Selftext    string  `json:"selftext"`
			Body        string  `json:"body"`
			Permalink   string  `json:"permalink"`
			Score       int     `json:"score"`
			NumComments int     `json:"num_comments"`
			CreatedUTC  float64 `json:"created_utc"`
		} `json:"data"`
	}
	child.Kind = kind
	child.Data.ID = id
	child.Data.Title = title
	child.Data.Selftext = selftext
	child.Data.Body = body
	child.Data.Permalink = permalink
	child.Data.Score = score
	child.Data.NumComments = numComments
	child.Data.CreatedUTC = createdUTC
	return child
}

func (f fakeSeenStore) Seen(ctx context.Context, providerStreamID string, externalIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(externalIDs))
	for _, id := range externalIDs {
		if f.known[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (f fakeSeenStore) MarkSeen(ctx context.Context, providerStreamID string, externalIDs []string) error {
	return nil
}

func TestRedditBuildJobBootstrapForNewSource(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.ProviderStream{
		ID:       "source-1",
		Name:     "golang",
		Provider: "reddit",
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
	job, err := s.BuildJob(db.ProviderStream{
		ID:       "source-1",
		Name:     "golang",
		Provider: "reddit",
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

	s, _ := newTestRedditSyncer(&fakeRedditClient{resp: func() *redditListingResponse {
		resp := redditResponse(
			redditFixturePost{"new-3", "newest", "", "/r/golang/comments/new3/x", 10, 3, 300},
			redditFixturePost{"new-2", "newer", "", "/r/golang/comments/new2/x", 9, 2, 200},
			redditFixturePost{"seen-1", "seen", "", "/r/golang/comments/seen1/x", 8, 1, 100},
		)
		resp.Data.After = "t3_next"
		return resp
	}()}, fakeSeenStore{known: map[string]bool{"t3_seen-1": true}})

	job := &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "reddit",
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
	if got := result.ProviderStreamUpdates["last_seen_id"]; got != "t3_new-3" {
		t.Fatalf("last_seen_id = %v, want t3_new-3", got)
	}
	if got := floatFromAny(result.ProviderStreamUpdates["last_seen_created_utc"]); got != 300 {
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

	s, _ := newTestRedditSyncer(&fakeRedditClient{resp: func() *redditListingResponse {
		resp := redditResponse(
			redditFixturePost{"new-3", "newest", "", "/r/golang/comments/new3/x", 10, 3, 300},
			redditFixturePost{"new-2", "newer", "", "/r/golang/comments/new2/x", 9, 2, 200},
		)
		resp.Data.After = "t3_next"
		return resp
	}()}, sync.NewNoopSeenStore())

	job := &db.SyncJob{
		CorrelationID:    "corr-1",
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		Priority:         5,
		MaxAttempts:      3,
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
	if got := result.ProviderStreamUpdates["last_seen_id"]; got != "t3_new-3" {
		t.Fatalf("last_seen_id = %v, want t3_new-3", got)
	}
	if got := result.HeadBoundary["id"]; got != "t3_new-3" {
		t.Fatalf("head_boundary.id = %v, want t3_new-3", got)
	}
}

func TestRedditExecuteDoesNotAdvanceHighWaterMarkWhenFirstPostIsSeen(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(fakeSeenStore{known: map[string]bool{"t3_seen-1": true}})
	s.limiter = &fakeRequestLimiter{}
	s.client = &fakeRedditClient{resp: func() *redditListingResponse {
		resp := redditResponse(
			redditFixturePost{"seen-1", "seen", "", "/r/golang/comments/seen1/x", 8, 1, 100},
		)
		resp.Data.After = "t3_next"
		return resp
	}()}

	job := &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "reddit",
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
	if _, ok := result.ProviderStreamUpdates["last_seen_id"]; ok {
		t.Fatal("last_seen_id should not advance when boundary is first post")
	}
	if len(result.HeadBoundary) != 0 {
		t.Fatalf("head_boundary = %v, want empty", result.HeadBoundary)
	}
}

func TestRedditExecuteEmitsCanonicalIDsAndProvenance(t *testing.T) {
	t.Parallel()

	s, _ := newTestRedditSyncer(&fakeRedditClient{resp: redditResponse(
		redditFixturePost{"abc123", "Pairs trading", "Cointegration notes", "/r/algotrading/comments/abc123/pairs", 17, 2, 420},
	)}, sync.NewNoopSeenStore())

	job := &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "reddit",
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

func TestRedditExecuteHydratesAcceptedPostsInline(t *testing.T) {
	t.Parallel()

	now := float64(timeNowUnix())
	client := &fakeRedditClient{
		resp: redditResponse(
			redditFixturePost{"post-1", "Post 1", "Body 1", "/r/golang/comments/post1/x", 120, 42, now},
			redditFixturePost{"post-2", "Post 2", "Body 2", "/r/golang/comments/post2/x", 20, 10, now},
			redditFixturePost{"seen-1", "Seen", "", "/r/golang/comments/seen1/x", 5, 1, now},
		),
		threads: map[string]*redditThreadResponse{
			"t3_post-1": redditThreadResponseFor(
				redditFixturePost{"post-1", "Post 1", "Body 1", "/r/golang/comments/post1/x", 120, 42, now},
				redditFixtureComment{"c1", "first useful comment", "/r/golang/comments/post1/x/c1", 9, now + 1},
			),
			"t3_post-2": redditThreadResponseFor(
				redditFixturePost{"post-2", "Post 2", "Body 2", "/r/golang/comments/post2/x", 20, 10, now},
				redditFixtureComment{"c2", "second useful comment", "/r/golang/comments/post2/x/c2", 4, now + 1},
			),
		},
	}
	s, limiter := newTestRedditSyncer(client, fakeSeenStore{known: map[string]bool{"t3_seen-1": true}})

	result, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload: map[string]any{
			"subreddit": "golang",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("record count = %d, want 2", len(result.Records))
	}
	if got, want := client.threadCalls, []string{"t3_post-1", "t3_post-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("thread calls = %v, want %v", got, want)
	}
	if limiter.calls != 3 {
		t.Fatalf("rate limiter calls = %d, want 3", limiter.calls)
	}
	if strings.Contains(result.Records[0].Content, "first useful comment") {
		t.Fatal("durable Content includes raw comment body")
	}
	if !strings.Contains(result.Records[0].EnrichmentContent, "first useful comment") {
		t.Fatalf("EnrichmentContent = %q, want hydrated comment context", result.Records[0].EnrichmentContent)
	}
	if got := result.Records[0].Metadata["top_comment_count"]; got != 1 {
		t.Fatalf("top_comment_count = %v, want 1", got)
	}
	if len(result.NewCycles) != 2 {
		t.Fatalf("new hydration cycles = %d, want 2", len(result.NewCycles))
	}
	if result.NewCycles[0].Kind != sync.CycleKindHydration || result.NewCycles[0].TargetExternalID != "t3_post-1" {
		t.Fatalf("first hydration cycle = %#v, want post-1 hydration", result.NewCycles[0])
	}
}

func TestRedditExecuteFailsWholePageWhenInlineHydrationFails(t *testing.T) {
	t.Parallel()

	s, _ := newTestRedditSyncer(&fakeRedditClient{
		resp: redditResponse(
			redditFixturePost{"post-1", "Post 1", "Body 1", "/r/golang/comments/post1/x", 120, 42, float64(timeNowUnix())},
		),
		threadErrs: map[string]error{"t3_post-1": errors.New("thread unavailable")},
	}, sync.NewNoopSeenStore())

	_, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "reddit",
		JobType:          "fetch_posts",
		Payload: map[string]any{
			"subreddit": "golang",
		},
	})
	if err == nil {
		t.Fatal("Execute returned nil, want hydration failure")
	}
}

func timeNowUnix() int64 {
	return time.Now().UTC().Unix()
}
