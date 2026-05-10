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
	redditAPI              = "https://www.reddit.com"
	userAgent              = "aladin-bot/0.1"
	redditRequestsPerMin   = 6
	redditListingPageLimit = 10

	QueueRedditHead = "sync_head:reddit"
	QueueReddit     = "sync:reddit"
)

type requestLimiter interface {
	Wait(ctx context.Context) error
}

type noopRequestLimiter struct{}

func (noopRequestLimiter) Wait(ctx context.Context) error { return nil }

type redditCycleState struct {
	Subreddit           string
	After               string
	PostID              string
	TopCommentsN        int
	NextLastSeenID      string
	NextLastSeenCreated float64
}

type redditListingResponse struct {
	Data struct {
		After    string `json:"after"`
		Children []struct {
			Data struct {
				ID          string  `json:"id"`
				Title       string  `json:"title"`
				Selftext    string  `json:"selftext"`
				Permalink   string  `json:"permalink"`
				Score       int     `json:"score"`
				NumComments int     `json:"num_comments"`
				CreatedUTC  float64 `json:"created_utc"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditThreadResponse []struct {
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

type redditComment struct {
	ExternalID string
	Body       string
	Permalink  string
	Score      int
	CreatedUTC float64
}

type RedditSyncer struct {
	client  RedditClient
	limiter requestLimiter
	seen    sync.SeenStore
}

func NewRedditSyncer(seen sync.SeenStore) *RedditSyncer {
	s := NewRedditSyncerWithClient(newRedditHTTPClient(), seen)
	s.limiter = ratelimit.NewPaced(redditRequestsPerMin)
	return s
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
		limiter: noopRequestLimiter{},
		seen:    seen,
	}
}

func (r *RedditSyncer) Provider() string  { return "reddit" }
func (r *RedditSyncer) HeadQueue() string { return QueueRedditHead }

func (r *RedditSyncer) BuildJob(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	subreddit, _ := stream.Config["subreddit"].(string)
	if subreddit == "" {
		return nil, fmt.Errorf("reddit stream %q missing required config field: subreddit", stream.Name)
	}

	state := newRedditCycleState(stream, cycle, subreddit)
	jobType := "fetch_posts"
	if cycle != nil && cycle.Kind == sync.CycleKindHydration {
		jobType = "fetch_hydrated_post"
		state.PostID = cycle.TargetExternalID
	}
	cycleID := ""
	if cycle != nil {
		cycleID = cycle.ID
	}
	correlationID := uuid.NewString()
	payload, err := json.Marshal(db.SyncJob{
		CorrelationID:    correlationID,
		CycleID:          cycleID,
		ProviderStreamID: stream.ID,
		Provider:         "reddit",
		JobType:          jobType,
		Payload:          state.payload(),
		Priority:         5,
		MaxAttempts:      3,
	})
	if err != nil {
		return nil, fmt.Errorf("reddit BuildJob marshal: %w", err)
	}

	return &db.ScheduledJob{
		ID:            stream.ID,
		CorrelationID: correlationID,
		Type:          "sync:reddit",
		Payload:       payload,
		Priority:      5,
		MaxRetry:      3,
		Timeout:       5 * time.Minute,
	}, nil
}
func (r *RedditSyncer) Execute(ctx context.Context, job *db.SyncJob) (*sync.Result, error) {
	state := redditCycleStateFromPayload(job.Payload)
	log := slog.With(
		"component", "source_syncer",
		"provider", "reddit",
		"provider_stream_id", job.ProviderStreamID,
		"cycle_id", job.CycleID,
		"correlation_id", job.CorrelationID,
		"subreddit", state.Subreddit,
	)
	if job.JobType == "fetch_hydrated_post" {
		return r.executeHydration(ctx, job, state, log)
	}

	log.Debug("reddit: fetching listing", "after", state.After, "has_next_last_seen", state.NextLastSeenID != "")
	body, err := r.fetchListing(ctx, state)
	if err != nil {
		return nil, err
	}
	log.Debug("reddit: fetched listing", "provider_record_count", len(body.Data.Children), "has_next_cursor", body.Data.After != "")

	result := &sync.Result{
		ProviderStreamUpdates: map[string]any{},
		CursorUpdates:         map[string]any{},
	}
	externalIDs := make([]string, 0, len(body.Data.Children))
	for _, child := range body.Data.Children {
		if child.Data.ID != "" {
			externalIDs = append(externalIDs, redditExternalID(child.Data.ID))
		}
	}
	known, err := r.seen.Seen(ctx, job.ProviderStreamID, externalIDs)
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

		content := redditPostContent(post.Title, post.Selftext)
		comments, err := r.fetchTopComments(ctx, state.Subreddit, externalID, topCommentsLimit(state.TopCommentsN))
		if err != nil {
			return nil, fmt.Errorf("reddit hydrate %s: %w", externalID, err)
		}
		enrichmentContent := redditHydratedContent(content, comments)
		commentIDs := make([]string, 0, len(comments))
		for _, comment := range comments {
			commentIDs = append(commentIDs, comment.ExternalID)
		}
		result.Records = append(result.Records, &sync.RawRecord{
			ExternalID:        externalID,
			SourceRevision:    redditSourceRevision(post.CreatedUTC, post.Score, post.NumComments),
			Type:              "post",
			Label:             post.Title,
			Content:           content,
			EnrichmentContent: enrichmentContent,
			SourceURL:         redditURL(post.Permalink),
			Metadata: map[string]any{
				"platform":          "reddit",
				"reddit_id":         externalID,
				"subreddit":         state.Subreddit,
				"permalink":         post.Permalink,
				"score":             post.Score,
				"num_comments":      post.NumComments,
				"hydrated":          true,
				"top_comment_count": len(comments),
				"top_comment_ids":   commentIDs,
				"created_utc":       post.CreatedUTC,
			},
		})
		if hydrationInterval := redditHydrationInterval(post.Score, post.NumComments, post.CreatedUTC, time.Now().UTC()); hydrationInterval != nil {
			result.NewCycles = append(result.NewCycles, &db.SyncCycle{
				Kind:             sync.CycleKindHydration,
				Status:           sync.CycleStatusActive,
				TargetKind:       "reddit_post",
				TargetExternalID: externalID,
				Cursor: map[string]any{
					"cycle_kind":                 sync.CycleKindHydration,
					"subreddit":                  state.Subreddit,
					"post_id":                    externalID,
					"top_comments_n":             topCommentsLimit(state.TopCommentsN),
					"hydration_interval_seconds": int(hydrationInterval.Seconds()),
				},
			})
		}
	}

	if newestAcceptedID != "" {
		result.HeadBoundary = map[string]any{
			"id":          newestAcceptedID,
			"created_utc": newestAcceptedCreated,
		}
	}

	if state.NextLastSeenID != "" {
		result.ProviderStreamUpdates["last_seen_id"] = state.NextLastSeenID
		result.ProviderStreamUpdates["last_seen_created_utc"] = state.NextLastSeenCreated
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
		"provider_stream_update_keys", mapKeysForLog(result.ProviderStreamUpdates),
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

func (r *RedditSyncer) executeHydration(ctx context.Context, job *db.SyncJob, state redditCycleState, log *slog.Logger) (*sync.Result, error) {
	postID := state.PostID
	if postID == "" {
		postID, _ = job.Payload["post_id"].(string)
	}
	if postID == "" {
		return nil, fmt.Errorf("reddit hydration missing post_id")
	}
	log.Debug("reddit: hydrating post", "post_id", postID)

	body, err := r.fetchThread(ctx, state.Subreddit, postID, topCommentsLimit(state.TopCommentsN))
	if err != nil {
		return nil, err
	}
	post, comments, err := redditThreadParts(body, postID, topCommentsLimit(state.TopCommentsN))
	if err != nil {
		return nil, err
	}
	externalID := redditExternalID(post.ID)
	content := redditPostContent(post.Title, post.Selftext)
	commentIDs := make([]string, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ExternalID)
	}

	result := &sync.Result{
		Records: []*sync.RawRecord{{
			ExternalID:        externalID,
			SourceRevision:    redditSourceRevision(post.CreatedUTC, post.Score, post.NumComments),
			Type:              "post",
			Label:             post.Title,
			Content:           content,
			EnrichmentContent: redditHydratedContent(content, comments),
			SourceURL:         redditURL(post.Permalink),
			Metadata: map[string]any{
				"platform":          "reddit",
				"reddit_id":         externalID,
				"subreddit":         state.Subreddit,
				"permalink":         post.Permalink,
				"score":             post.Score,
				"num_comments":      post.NumComments,
				"hydrated":          true,
				"rehydrated":        true,
				"top_comment_count": len(comments),
				"top_comment_ids":   commentIDs,
				"created_utc":       post.CreatedUTC,
			},
		}},
		ProviderStreamUpdates: map[string]any{},
		CursorUpdates: map[string]any{
			"cycle_kind":     sync.CycleKindHydration,
			"subreddit":      state.Subreddit,
			"post_id":        externalID,
			"top_comments_n": topCommentsLimit(state.TopCommentsN),
		},
		HeadBoundary: map[string]any{"id": externalID},
	}
	if hydrationInterval := redditHydrationInterval(post.Score, post.NumComments, post.CreatedUTC, time.Now().UTC()); hydrationInterval != nil {
		result.HasMore = true
		result.CursorUpdates["hydration_interval_seconds"] = int(hydrationInterval.Seconds())
	} else {
		result.CompletionReason = sync.CompletionReasonCold
	}
	return result, nil
}

func (r *RedditSyncer) fetchTopComments(ctx context.Context, subreddit string, postID string, limit int) ([]redditComment, error) {
	body, err := r.fetchThread(ctx, subreddit, postID, limit)
	if err != nil {
		return nil, err
	}
	_, comments, err := redditThreadParts(body, postID, limit)
	return comments, err
}

func (r *RedditSyncer) fetchListing(ctx context.Context, state redditCycleState) (*redditListingResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("reddit rate limit wait: %w", err)
	}
	return r.client.FetchListing(ctx, state)
}

func (r *RedditSyncer) fetchThread(ctx context.Context, subreddit string, postID string, limit int) (*redditThreadResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("reddit thread rate limit wait: %w", err)
	}
	return r.client.FetchThread(ctx, subreddit, postID, limit)
}

func redditThreadParts(body *redditThreadResponse, postID string, limit int) (struct {
	ID          string
	Title       string
	Selftext    string
	Permalink   string
	Score       int
	NumComments int
	CreatedUTC  float64
}, []redditComment, error) {
	var post struct {
		ID          string
		Title       string
		Selftext    string
		Permalink   string
		Score       int
		NumComments int
		CreatedUTC  float64
	}
	if body == nil || len(*body) == 0 {
		return post, nil, fmt.Errorf("reddit thread %s empty response", postID)
	}
	if len((*body)[0].Data.Children) == 0 {
		return post, nil, fmt.Errorf("reddit thread %s missing post", postID)
	}
	postChild := (*body)[0].Data.Children[0].Data
	post.ID = redditExternalID(postChild.ID)
	post.Title = postChild.Title
	post.Selftext = postChild.Selftext
	post.Permalink = postChild.Permalink
	post.Score = postChild.Score
	post.NumComments = postChild.NumComments
	post.CreatedUTC = postChild.CreatedUTC

	if len(*body) < 2 {
		return post, nil, nil
	}
	comments := make([]redditComment, 0, limit)
	for _, child := range (*body)[1].Data.Children {
		if child.Kind != "t1" || child.Data.ID == "" || child.Data.Body == "" {
			continue
		}
		comments = append(comments, redditComment{
			ExternalID: redditCommentExternalID(child.Data.ID),
			Body:       child.Data.Body,
			Permalink:  child.Data.Permalink,
			Score:      child.Data.Score,
			CreatedUTC: child.Data.CreatedUTC,
		})
		if len(comments) >= limit {
			break
		}
	}
	return post, comments, nil
}

func redditPostContent(title string, selftext string) string {
	content := title
	if selftext != "" {
		content += "\n\n" + selftext
	}
	return content
}

func redditHydratedContent(postContent string, comments []redditComment) string {
	if len(comments) == 0 {
		return postContent
	}
	var b strings.Builder
	b.WriteString(postContent)
	b.WriteString("\n\nTop comments:")
	for _, comment := range comments {
		b.WriteString("\n\n- ")
		b.WriteString(comment.Body)
	}
	return b.String()
}

func redditCommentExternalID(id string) string {
	if id == "" || strings.HasPrefix(id, "t1_") {
		return id
	}
	return "t1_" + id
}

func topCommentsLimit(configured int) int {
	if configured <= 0 {
		return 5
	}
	if configured > 20 {
		return 20
	}
	return configured
}

func redditSourceRevision(createdUTC float64, score int, numComments int) int64 {
	return int64(createdUTC)*1_000_000 + int64(score)*1_000 + int64(numComments)
}

func redditHydrationInterval(score int, numComments int, createdUTC float64, now time.Time) *time.Duration {
	created := time.Unix(int64(createdUTC), 0).UTC()
	age := now.Sub(created)
	if age < 0 {
		age = 0
	}
	var interval time.Duration
	switch {
	case (score >= 500 || numComments >= 100) && age < 24*time.Hour:
		interval = 30 * time.Minute
	case (score >= 100 || numComments >= 40) && age < 12*time.Hour:
		interval = time.Hour
	case (score >= 20 || numComments >= 10) && age < 6*time.Hour:
		interval = 2 * time.Hour
	default:
		return nil
	}
	return &interval
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

func newRedditCycleState(stream db.ProviderStream, cycle *db.SyncCycle, subreddit string) redditCycleState {
	state := redditCycleState{
		Subreddit:    subreddit,
		TopCommentsN: int(floatFromAny(stream.Config["top_comments_n"])),
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
	state.PostID, _ = payload["post_id"].(string)
	state.TopCommentsN = int(floatFromAny(payload["top_comments_n"]))
	state.NextLastSeenID, _ = payload["next_last_seen_id"].(string)
	state.NextLastSeenCreated = floatFromAny(payload["next_last_seen_created_utc"])
	return state
}

func (s redditCycleState) payload() map[string]any {
	return map[string]any{
		"subreddit":                  s.Subreddit,
		"after":                      s.After,
		"post_id":                    s.PostID,
		"top_comments_n":             topCommentsLimit(s.TopCommentsN),
		"cycle_kind":                 cycleKind(s),
		"next_last_seen_id":          s.NextLastSeenID,
		"next_last_seen_created_utc": s.NextLastSeenCreated,
	}
}

func cycleKind(s redditCycleState) string {
	if s.PostID != "" {
		return sync.CycleKindHydration
	}
	return sync.CycleKindRefresh
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
