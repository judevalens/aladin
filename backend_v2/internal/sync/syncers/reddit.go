package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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
	client  RedditClient
	limiter *ratelimit.Limiter
	seen    sync.SeenStore
}

func NewRedditSyncer(seen sync.SeenStore) *RedditSyncer {
	return NewRedditSyncerWithClient(newRedditHTTPClient(), seen)
}

func NewRedditSyncerWithClient(client RedditClient, seen sync.SeenStore) *RedditSyncer {
	if seen == nil {
		seen = sync.NewNoopSeenStore()
	}
	if client == nil {
		client = newRedditHTTPClient()
	}
	return &RedditSyncer{
		client:  client,
		limiter: ratelimit.New(rateLimit),
		seen:    seen,
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
	cycleID := ""
	if cycle != nil {
		cycleID = cycle.ID
	}
	correlationID := uuid.NewString()
	payload, err := json.Marshal(db.SyncJob{
		CorrelationID: correlationID,
		CycleID:       cycleID,
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
	log := slog.With(
		"component", "source_syncer",
		"source_type", "reddit",
		"source_id", job.SourceID,
		"cycle_id", job.CycleID,
		"correlation_id", job.CorrelationID,
		"subreddit", state.Subreddit,
	)
	log.Debug("reddit: fetching listing", "after", state.After, "has_next_last_seen", state.NextLastSeenID != "")
	body, err := r.client.FetchListing(ctx, state)
	if err != nil {
		return nil, err
	}
	log.Debug("reddit: fetched listing", "provider_record_count", len(body.Data.Children), "has_next_cursor", body.Data.After != "")

	result := &sync.Result{
		SourceUpdates: map[string]any{},
		CursorUpdates: map[string]any{},
	}
	externalIDs := make([]string, 0, len(body.Data.Children))
	for _, child := range body.Data.Children {
		if child.Data.ID != "" {
			externalIDs = append(externalIDs, redditExternalID(child.Data.ID))
		}
	}
	known, err := r.seen.Seen(ctx, job.SourceID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("reddit seen lookup: %w", err)
	}
	log.Debug("reddit: seen lookup complete", "candidate_count", len(externalIDs), "known_count", countSeen(known))
	hitBoundary := false
	var newestAcceptedID string
	var newestAcceptedCreated float64
	for _, child := range body.Data.Children {
		post := child.Data
		externalID := redditExternalID(post.ID)
		if known[externalID] {
			log.Debug("reddit: seen boundary reached", "external_id", externalID)
			hitBoundary = true
			result.CompletionReason = sync.CompletionReasonSeenOverlap
			break
		}

		if state.NextLastSeenID == "" {
			state.NextLastSeenID = externalID
			state.NextLastSeenCreated = post.CreatedUTC
		}
		if newestAcceptedID == "" {
			newestAcceptedID = externalID
			newestAcceptedCreated = post.CreatedUTC
		}

		content := post.Title
		if post.Selftext != "" {
			content += "\n\n" + post.Selftext
		}
		result.Records = append(result.Records, &sync.RawRecord{
			ExternalID: externalID,
			Type:       "post",
			Label:      post.Title,
			Content:    content,
			SourceURL:  redditURL(post.Permalink),
			Metadata: map[string]any{
				"platform":    "reddit",
				"reddit_id":   externalID,
				"subreddit":   state.Subreddit,
				"permalink":   post.Permalink,
				"score":       post.Score,
				"created_utc": post.CreatedUTC,
			},
		})
	}

	if newestAcceptedID != "" {
		result.HeadBoundary = map[string]any{
			"id":          newestAcceptedID,
			"created_utc": newestAcceptedCreated,
		}
	}

	if state.NextLastSeenID != "" {
		result.SourceUpdates["last_seen_id"] = state.NextLastSeenID
		result.SourceUpdates["last_seen_created_utc"] = state.NextLastSeenCreated
	}

	if !hitBoundary && body.Data.After != "" {
		result.HasMore = true
		result.CursorUpdates["after"] = body.Data.After
	} else {
		if result.CompletionReason == "" {
			result.CompletionReason = sync.CompletionReasonExhausted
		}
		result.CursorUpdates["after"] = ""
	}

	log.Debug(
		"reddit: execution result built",
		"accepted_record_count", len(result.Records),
		"has_more", result.HasMore,
		"completion_reason", result.CompletionReason,
		"source_update_keys", mapKeysForLog(result.SourceUpdates),
		"cursor_update_keys", mapKeysForLog(result.CursorUpdates),
		"head_boundary_keys", mapKeysForLog(result.HeadBoundary),
	)
	return result, nil
}

func redditExternalID(id string) string {
	if id == "" || strings.HasPrefix(id, "t3_") {
		return id
	}
	return "t3_" + id
}

func redditURL(permalink string) string {
	if strings.HasPrefix(permalink, "http://") || strings.HasPrefix(permalink, "https://") {
		return permalink
	}
	return redditAPI + permalink
}

func countSeen(known map[string]bool) int {
	count := 0
	for _, seen := range known {
		if seen {
			count++
		}
	}
	return count
}

func newRedditCycleState(source db.Source, cycle *db.SyncCycle, subreddit string) redditCycleState {
	state := redditCycleState{
		Subreddit: subreddit,
	}
	if cycle != nil && len(cycle.Cursor) > 0 {
		state = redditCycleStateFromPayload(cycle.Cursor)
		if state.Subreddit == "" {
			state.Subreddit = subreddit
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
	state.NextLastSeenID, _ = payload["next_last_seen_id"].(string)
	state.NextLastSeenCreated = floatFromAny(payload["next_last_seen_created_utc"])
	return state
}

func (s redditCycleState) payload() map[string]any {
	return map[string]any{
		"subreddit":                  s.Subreddit,
		"after":                      s.After,
		"next_last_seen_id":          s.NextLastSeenID,
		"next_last_seen_created_utc": s.NextLastSeenCreated,
	}
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
