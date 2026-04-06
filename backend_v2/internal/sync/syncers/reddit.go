package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/sync"
)

const (
	redditAPI   = "https://www.reddit.com"
	userAgent   = "aladin-bot/0.1"
	rateLimit   = 10 // requests per minute
	redditLimit = 25

	QueueRedditHead = "sync_head:reddit"
	QueueReddit     = "sync:reddit"
)

type redditCycleState struct {
	Subreddit           string
	After               string
	StopBeforeID        string
	StopBeforeCreated   float64
	NextLastSeenID      string
	NextLastSeenCreated float64
}

type redditListingResponse struct {
	Data struct {
		After    string `json:"after"`
		Children []struct {
			Data struct {
				ID         string  `json:"id"`
				Title      string  `json:"title"`
				Selftext   string  `json:"selftext"`
				Permalink  string  `json:"permalink"`
				Score      int     `json:"score"`
				CreatedUTC float64 `json:"created_utc"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type RedditSyncer struct {
	client  *http.Client
	limiter *ratelimit.Limiter
}

func NewRedditSyncer() *RedditSyncer {
	return &RedditSyncer{
		client:  &http.Client{Timeout: 10 * time.Second},
		limiter: ratelimit.New(rateLimit),
	}
}

func (r *RedditSyncer) SourceType() string { return "reddit" }
func (r *RedditSyncer) HeadQueue() string  { return QueueRedditHead }

func (r *RedditSyncer) BuildJob(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	subreddit, _ := source.Config["subreddit"].(string)
	if subreddit == "" {
		return nil, fmt.Errorf("reddit source %q missing required config field: subreddit", source.Name)
	}

	state := newRedditCycleState(source, cycle, subreddit)
	correlationID := uuid.NewString()
	payload, err := json.Marshal(db.SyncJob{
		CorrelationID: correlationID,
		CycleID:       cycle.ID,
		SourceID:      source.ID,
		KgID:          source.KgID,
		SourceType:    "reddit",
		JobType:       "fetch_posts",
		Payload:       state.payload(),
		Priority:      5,
		MaxAttempts:   3,
	})
	if err != nil {
		return nil, fmt.Errorf("reddit BuildJob marshal: %w", err)
	}

	return &db.ScheduledJob{
		ID:            source.ID,
		CorrelationID: correlationID,
		Type:          "sync:reddit",
		Payload:       payload,
		Priority:      5,
		MaxRetry:      3,
		Timeout:       5 * time.Minute,
	}, nil
}
func (r *RedditSyncer) Execute(ctx context.Context, job *db.SyncJob) (*sync.Result, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("reddit rate limit wait: %w", err)
	}

	state := redditCycleStateFromPayload(job.Payload)
	body, err := r.fetchListing(ctx, state)
	if err != nil {
		return nil, err
	}

	result := &sync.Result{
		SourceUpdates: map[string]any{},
		CursorUpdates: map[string]any{},
	}
	hitBoundary := false
	for _, child := range body.Data.Children {
		post := child.Data
		if reachedBoundary(state, post.ID, post.CreatedUTC) {
			hitBoundary = true
			break
		}

		if state.NextLastSeenID == "" {
			state.NextLastSeenID = post.ID
			state.NextLastSeenCreated = post.CreatedUTC
		}

		content := post.Title
		if post.Selftext != "" {
			content += "\n\n" + post.Selftext
		}
		result.Artifacts = append(result.Artifacts, &sync.RawArtifact{
			ExternalID: post.ID,
			Type:       "post",
			Label:      post.Title,
			Content:    content,
			SourceURL:  redditAPI + post.Permalink,
			Metadata:   map[string]any{"score": post.Score},
		})
	}

	if state.NextLastSeenID != "" {
		result.SourceUpdates["last_seen_id"] = state.NextLastSeenID
		result.SourceUpdates["last_seen_created_utc"] = state.NextLastSeenCreated
	}

	if !hitBoundary && body.Data.After != "" {
		result.HasMore = true
		result.CursorUpdates["after"] = body.Data.After
	} else {
		result.CursorUpdates["after"] = ""
	}

	return result, nil
}

func (r *RedditSyncer) fetchListing(ctx context.Context, state redditCycleState) (*redditListingResponse, error) {
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

func newRedditCycleState(source db.Source, cycle *db.SyncCycle, subreddit string) redditCycleState {
	lastSeenID, _ := source.Config["last_seen_id"].(string)
	lastSeenCreated := floatFromAny(source.Config["last_seen_created_utc"])
	state := redditCycleState{
		Subreddit:         subreddit,
		StopBeforeID:      lastSeenID,
		StopBeforeCreated: lastSeenCreated,
	}
	if cycle != nil && len(cycle.Cursor) > 0 {
		state = redditCycleStateFromPayload(cycle.Cursor)
		if state.Subreddit == "" {
			state.Subreddit = subreddit
		}
		if state.StopBeforeID == "" {
			state.StopBeforeID = lastSeenID
			state.StopBeforeCreated = lastSeenCreated
		}
		return state
	}
	return state
}

func redditCycleStateFromPayload(payload map[string]any) redditCycleState {
	var state redditCycleState
	if payload == nil {
		return state
	}
	state.Subreddit, _ = payload["subreddit"].(string)
	state.After, _ = payload["after"].(string)
	state.StopBeforeID, _ = payload["stop_before_id"].(string)
	state.StopBeforeCreated = floatFromAny(payload["stop_before_created_utc"])
	state.NextLastSeenID, _ = payload["next_last_seen_id"].(string)
	state.NextLastSeenCreated = floatFromAny(payload["next_last_seen_created_utc"])
	return state
}

func (s redditCycleState) payload() map[string]any {
	return map[string]any{
		"subreddit":                  s.Subreddit,
		"after":                      s.After,
		"stop_before_id":             s.StopBeforeID,
		"stop_before_created_utc":    s.StopBeforeCreated,
		"next_last_seen_id":          s.NextLastSeenID,
		"next_last_seen_created_utc": s.NextLastSeenCreated,
	}
}

func reachedBoundary(state redditCycleState, id string, createdUTC float64) bool {
	if state.StopBeforeID == "" && state.StopBeforeCreated == 0 {
		return false
	}
	if state.StopBeforeID != "" && id == state.StopBeforeID {
		return true
	}
	return state.StopBeforeCreated > 0 && createdUTC <= state.StopBeforeCreated
}

func floatFromAny(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}
