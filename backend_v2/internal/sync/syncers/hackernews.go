package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/sync"
)

const (
	hackerNewsAPI              = "https://hacker-news.firebaseio.com"
	hackerNewsRequestsPerMin   = 60
	hackerNewsListingPageLimit = 10

	QueueHackerNewsHead = "sync_head:hackernews"
	QueueHackerNews     = "sync:hackernews"
)

type hackerNewsCycleState struct {
	Feed                    string
	Offset                  int
	PageSize                int
	ItemID                  int64
	TopCommentsN            int
	NextLastSeenID          string
	NextLastSeenCreatedUnix int64
}

type hackerNewsItem struct {
	ID          int64   `json:"id"`
	Deleted     bool    `json:"deleted"`
	Type        string  `json:"type"`
	By          string  `json:"by"`
	Time        int64   `json:"time"`
	Text        string  `json:"text"`
	Dead        bool    `json:"dead"`
	Parent      int64   `json:"parent"`
	Poll        int64   `json:"poll"`
	Kids        []int64 `json:"kids"`
	URL         string  `json:"url"`
	Score       int     `json:"score"`
	Title       string  `json:"title"`
	Descendants int     `json:"descendants"`
}

type hackerNewsComment struct {
	ID     int64
	By     string
	Time   int64
	Text   string
	Parent int64
}

type HackerNewsSyncer struct {
	client  HackerNewsClient
	limiter requestLimiter
	seen    sync.SeenStore
}

func NewHackerNewsSyncer(seen sync.SeenStore) *HackerNewsSyncer {
	s := NewHackerNewsSyncerWithClient(newHackerNewsHTTPClient(), seen)
	s.limiter = ratelimit.NewPaced(hackerNewsRequestsPerMin)
	return s
}

func NewHackerNewsSyncerWithClient(client HackerNewsClient, seen sync.SeenStore) *HackerNewsSyncer {
	if seen == nil {
		seen = sync.NewNoopSeenStore()
	}
	if client == nil {
		client = newHackerNewsHTTPClient()
	}
	return &HackerNewsSyncer{
		client:  client,
		limiter: noopRequestLimiter{},
		seen:    seen,
	}
}

func (h *HackerNewsSyncer) Provider() string  { return "hackernews" }
func (h *HackerNewsSyncer) HeadQueue() string { return QueueHackerNewsHead }

func (h *HackerNewsSyncer) BuildJob(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	feed := hackerNewsFeedFromConfig(stream.Config)
	state := newHackerNewsCycleState(stream, cycle, feed)
	jobType := "fetch_stories"
	if cycle != nil && cycle.Kind == sync.CycleKindHydration {
		jobType = "fetch_hydrated_item"
		state.ItemID = hackerNewsIDFromExternal(cycle.TargetExternalID)
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
		Provider:         "hackernews",
		JobType:          jobType,
		Payload:          state.payload(),
		Priority:         5,
		MaxAttempts:      3,
	})
	if err != nil {
		return nil, fmt.Errorf("hackernews BuildJob marshal: %w", err)
	}

	return &db.ScheduledJob{
		ID:            stream.ID,
		CorrelationID: correlationID,
		Type:          "sync:hackernews",
		Payload:       payload,
		Priority:      5,
		MaxRetry:      3,
		Timeout:       5 * time.Minute,
	}, nil
}

func (h *HackerNewsSyncer) Execute(ctx context.Context, job *db.SyncJob) (*sync.Result, error) {
	state := hackerNewsCycleStateFromPayload(job.Payload)
	log := slog.With(
		"component", "source_syncer",
		"provider", "hackernews",
		"provider_stream_id", job.ProviderStreamID,
		"cycle_id", job.CycleID,
		"correlation_id", job.CorrelationID,
		"feed", state.Feed,
	)
	if job.JobType == "fetch_hydrated_item" {
		return h.executeHydration(ctx, job, state, log)
	}

	log.Debug("hackernews: fetching story ids", "offset", state.Offset, "page_size", state.pageSize())
	ids, err := h.fetchStoryIDs(ctx, state.Feed)
	if err != nil {
		return nil, err
	}
	pageIDs := hackerNewsPageIDs(ids, state.Offset, state.pageSize())
	log.Debug("hackernews: fetched story id page", "provider_story_count", len(ids), "page_record_count", len(pageIDs))

	result := &sync.Result{
		ProviderStreamUpdates: map[string]any{},
		CursorUpdates:         map[string]any{},
	}
	externalIDs := make([]string, 0, len(pageIDs))
	for _, id := range pageIDs {
		externalIDs = append(externalIDs, hackerNewsExternalID(id))
	}
	known, err := h.seen.Seen(ctx, job.ProviderStreamID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("hackernews seen lookup: %w", err)
	}

	hitBoundary := false
	var newestAcceptedID string
	var newestAcceptedTime int64
	for _, id := range pageIDs {
		externalID := hackerNewsExternalID(id)
		if known[externalID] {
			log.Debug("hackernews: seen boundary reached", "external_id", externalID)
			hitBoundary = true
			result.CompletionReason = sync.CompletionReasonSeenOverlap
			break
		}

		story, err := h.fetchItem(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("hackernews fetch story %d: %w", id, err)
		}
		if !hackerNewsAcceptStory(story) {
			continue
		}
		comments, err := h.fetchTopComments(ctx, story, state.topCommentsN())
		if err != nil {
			return nil, fmt.Errorf("hackernews hydrate %s: %w", externalID, err)
		}

		if state.NextLastSeenID == "" {
			state.NextLastSeenID = externalID
			state.NextLastSeenCreatedUnix = story.Time
		}
		if newestAcceptedID == "" {
			newestAcceptedID = externalID
			newestAcceptedTime = story.Time
		}
		result.Records = append(result.Records, hackerNewsRawRecord(state.Feed, story, comments, false))
		if hydrationInterval := hackerNewsHydrationInterval(story.Score, story.Descendants, story.Time, time.Now().UTC()); hydrationInterval != nil {
			result.NewCycles = append(result.NewCycles, &db.SyncCycle{
				Kind:             sync.CycleKindHydration,
				Status:           sync.CycleStatusActive,
				TargetKind:       "hackernews_item",
				TargetExternalID: externalID,
				Cursor: map[string]any{
					"cycle_kind":                 sync.CycleKindHydration,
					"feed":                       state.Feed,
					"item_id":                    story.ID,
					"top_comments_n":             state.topCommentsN(),
					"hydration_interval_seconds": int(hydrationInterval.Seconds()),
				},
			})
		}
	}

	if newestAcceptedID != "" {
		result.HeadBoundary = map[string]any{
			"id":        newestAcceptedID,
			"time_unix": newestAcceptedTime,
		}
	}
	if state.NextLastSeenID != "" {
		result.ProviderStreamUpdates["last_seen_item_id"] = state.NextLastSeenID
		result.ProviderStreamUpdates["last_seen_item_time"] = state.NextLastSeenCreatedUnix
	}

	nextOffset := state.Offset + len(pageIDs)
	if !hitBoundary && nextOffset < len(ids) && len(pageIDs) > 0 {
		result.HasMore = true
		result.CursorUpdates["offset"] = nextOffset
	} else {
		if result.CompletionReason == "" {
			result.CompletionReason = sync.CompletionReasonExhausted
		}
		result.CursorUpdates["offset"] = 0
	}

	log.Debug(
		"hackernews: execution result built",
		"accepted_record_count", len(result.Records),
		"has_more", result.HasMore,
		"completion_reason", result.CompletionReason,
		"provider_stream_update_keys", mapKeysForLog(result.ProviderStreamUpdates),
		"cursor_update_keys", mapKeysForLog(result.CursorUpdates),
		"head_boundary_keys", mapKeysForLog(result.HeadBoundary),
	)
	return result, nil
}

func (h *HackerNewsSyncer) executeHydration(ctx context.Context, job *db.SyncJob, state hackerNewsCycleState, log *slog.Logger) (*sync.Result, error) {
	itemID := state.ItemID
	if itemID == 0 {
		itemID = int64(floatFromAny(job.Payload["item_id"]))
	}
	if itemID == 0 {
		return nil, fmt.Errorf("hackernews hydration missing item_id")
	}
	log.Debug("hackernews: hydrating item", "item_id", itemID)

	story, err := h.fetchItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if !hackerNewsAcceptStory(story) {
		return &sync.Result{
			ProviderStreamUpdates: map[string]any{},
			CursorUpdates:         map[string]any{},
			HeadBoundary:          map[string]any{"id": hackerNewsExternalID(itemID)},
			CompletionReason:      sync.CompletionReasonCold,
		}, nil
	}
	comments, err := h.fetchTopComments(ctx, story, state.topCommentsN())
	if err != nil {
		return nil, err
	}

	result := &sync.Result{
		Records:               []*sync.RawRecord{hackerNewsRawRecord(state.Feed, story, comments, true)},
		ProviderStreamUpdates: map[string]any{},
		CursorUpdates: map[string]any{
			"cycle_kind":     sync.CycleKindHydration,
			"feed":           state.Feed,
			"item_id":        story.ID,
			"top_comments_n": state.topCommentsN(),
		},
		HeadBoundary: map[string]any{"id": hackerNewsExternalID(story.ID)},
	}
	if hydrationInterval := hackerNewsHydrationInterval(story.Score, story.Descendants, story.Time, time.Now().UTC()); hydrationInterval != nil {
		result.HasMore = true
		result.CursorUpdates["hydration_interval_seconds"] = int(hydrationInterval.Seconds())
	} else {
		result.CompletionReason = sync.CompletionReasonCold
	}
	return result, nil
}

func (h *HackerNewsSyncer) fetchStoryIDs(ctx context.Context, feed string) ([]int64, error) {
	if err := h.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("hackernews rate limit wait: %w", err)
	}
	return h.client.FetchStoryIDs(ctx, feed)
}

func (h *HackerNewsSyncer) fetchItem(ctx context.Context, id int64) (*hackerNewsItem, error) {
	if err := h.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("hackernews item rate limit wait: %w", err)
	}
	return h.client.FetchItem(ctx, id)
}

func (h *HackerNewsSyncer) fetchTopComments(ctx context.Context, story *hackerNewsItem, limit int) ([]hackerNewsComment, error) {
	if story == nil || limit <= 0 {
		return nil, nil
	}
	commentIDs := story.Kids
	if len(commentIDs) > limit {
		commentIDs = commentIDs[:limit]
	}
	comments := make([]hackerNewsComment, 0, len(commentIDs))
	for _, id := range commentIDs {
		item, err := h.fetchItem(ctx, id)
		if err != nil {
			return nil, err
		}
		if !hackerNewsAcceptComment(item) {
			continue
		}
		comments = append(comments, hackerNewsComment{
			ID:     item.ID,
			By:     item.By,
			Time:   item.Time,
			Text:   hackerNewsCleanText(item.Text),
			Parent: item.Parent,
		})
	}
	return comments, nil
}

func hackerNewsRawRecord(feed string, story *hackerNewsItem, comments []hackerNewsComment, rehydrated bool) *sync.RawRecord {
	content := hackerNewsStoryContent(story)
	commentIDs := make([]int64, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}
	metadata := map[string]any{
		"platform":          "hackernews",
		"feed":              feed,
		"hn_id":             story.ID,
		"by":                story.By,
		"time_unix":         story.Time,
		"score":             story.Score,
		"descendants":       story.Descendants,
		"url":               story.URL,
		"item_url":          hackerNewsItemURL(story.ID),
		"hydrated":          true,
		"top_comment_count": len(comments),
		"top_comment_ids":   commentIDs,
	}
	if rehydrated {
		metadata["rehydrated"] = true
	}
	return &sync.RawRecord{
		ExternalID:        hackerNewsExternalID(story.ID),
		SourceRevision:    hackerNewsSourceRevision(story.Time, story.Score, story.Descendants),
		Type:              "post",
		Label:             story.Title,
		Content:           content,
		EnrichmentContent: hackerNewsHydratedContent(content, comments),
		SourceURL:         hackerNewsSourceURL(story),
		Metadata:          metadata,
	}
}

func hackerNewsFeedFromConfig(config map[string]any) string {
	feed, _ := config["feed"].(string)
	feed = strings.TrimSpace(feed)
	switch feed {
	case "", "topstories", "newstories", "beststories", "askstories", "showstories", "jobstories":
		if feed == "" {
			return "topstories"
		}
		return feed
	default:
		return "topstories"
	}
}

func newHackerNewsCycleState(stream db.ProviderStream, cycle *db.SyncCycle, feed string) hackerNewsCycleState {
	state := hackerNewsCycleState{
		Feed:         feed,
		PageSize:     int(floatFromAny(stream.Config["page_size"])),
		TopCommentsN: int(floatFromAny(stream.Config["top_comments_n"])),
	}
	if cycle != nil && len(cycle.Cursor) > 0 {
		state = hackerNewsCycleStateFromPayload(cycle.Cursor)
		if state.Feed == "" {
			state.Feed = feed
		}
		return state
	}
	return state
}

func hackerNewsCycleStateFromPayload(payload map[string]any) hackerNewsCycleState {
	var state hackerNewsCycleState
	if payload == nil {
		return state
	}
	state.Feed, _ = payload["feed"].(string)
	state.Offset = int(floatFromAny(payload["offset"]))
	state.PageSize = int(floatFromAny(payload["page_size"]))
	state.ItemID = int64(floatFromAny(payload["item_id"]))
	state.TopCommentsN = int(floatFromAny(payload["top_comments_n"]))
	state.NextLastSeenID, _ = payload["next_last_seen_item_id"].(string)
	state.NextLastSeenCreatedUnix = int64(floatFromAny(payload["next_last_seen_item_time"]))
	return state
}

func (s hackerNewsCycleState) payload() map[string]any {
	payload := map[string]any{
		"feed":                     s.Feed,
		"offset":                   s.Offset,
		"page_size":                s.pageSize(),
		"top_comments_n":           s.topCommentsN(),
		"next_last_seen_item_id":   s.NextLastSeenID,
		"next_last_seen_item_time": s.NextLastSeenCreatedUnix,
	}
	if s.ItemID != 0 {
		payload["cycle_kind"] = sync.CycleKindHydration
		payload["item_id"] = s.ItemID
	}
	return payload
}

func (s hackerNewsCycleState) pageSize() int {
	if s.PageSize <= 0 {
		return hackerNewsListingPageLimit
	}
	if s.PageSize > 30 {
		return 30
	}
	return s.PageSize
}

func (s hackerNewsCycleState) topCommentsN() int {
	if s.TopCommentsN <= 0 {
		return 5
	}
	if s.TopCommentsN > 20 {
		return 20
	}
	return s.TopCommentsN
}

func hackerNewsPageIDs(ids []int64, offset int, pageSize int) []int64 {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = hackerNewsListingPageLimit
	}
	if offset >= len(ids) {
		return nil
	}
	end := offset + pageSize
	if end > len(ids) {
		end = len(ids)
	}
	return ids[offset:end]
}

func hackerNewsAcceptStory(item *hackerNewsItem) bool {
	return item != nil && !item.Deleted && !item.Dead && item.Type == "story" && item.ID != 0 && item.Title != ""
}

func hackerNewsAcceptComment(item *hackerNewsItem) bool {
	return item != nil && !item.Deleted && !item.Dead && item.Type == "comment" && item.ID != 0 && item.Text != ""
}

func hackerNewsExternalID(id int64) string {
	if id <= 0 {
		return ""
	}
	return "hn:" + strconv.FormatInt(id, 10)
}

func hackerNewsIDFromExternal(externalID string) int64 {
	externalID = strings.TrimPrefix(externalID, "hn:")
	id, _ := strconv.ParseInt(externalID, 10, 64)
	return id
}

func hackerNewsStoryContent(story *hackerNewsItem) string {
	if story == nil {
		return ""
	}
	parts := []string{story.Title}
	if story.URL != "" {
		parts = append(parts, story.URL)
	}
	if text := hackerNewsCleanText(story.Text); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func hackerNewsHydratedContent(storyContent string, comments []hackerNewsComment) string {
	if len(comments) == 0 {
		return storyContent
	}
	comments = append([]hackerNewsComment(nil), comments...)
	sort.SliceStable(comments, func(i, j int) bool {
		return comments[i].ID < comments[j].ID
	})
	var b strings.Builder
	b.WriteString(storyContent)
	b.WriteString("\n\nTop comments:")
	for _, comment := range comments {
		if comment.Text == "" {
			continue
		}
		b.WriteString("\n\n- ")
		if comment.By != "" {
			b.WriteString(comment.By)
			b.WriteString(": ")
		}
		b.WriteString(comment.Text)
	}
	return b.String()
}

func hackerNewsCleanText(text string) string {
	text = strings.ReplaceAll(text, "<p>", "\n\n")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func hackerNewsSourceURL(story *hackerNewsItem) string {
	if story == nil {
		return ""
	}
	if story.URL != "" {
		return story.URL
	}
	return hackerNewsItemURL(story.ID)
}

func hackerNewsItemURL(id int64) string {
	return "https://news.ycombinator.com/item?id=" + strconv.FormatInt(id, 10)
}

func hackerNewsSourceRevision(createdUnix int64, score int, descendants int) int64 {
	return createdUnix*1_000_000 + int64(score)*1_000 + int64(descendants)
}

func hackerNewsHydrationInterval(score int, descendants int, createdUnix int64, now time.Time) *time.Duration {
	created := time.Unix(createdUnix, 0).UTC()
	age := now.Sub(created)
	if age < 0 {
		age = 0
	}
	var interval time.Duration
	switch {
	case (score >= 300 || descendants >= 150) && age < 24*time.Hour:
		interval = 30 * time.Minute
	case (score >= 100 || descendants >= 50) && age < 12*time.Hour:
		interval = time.Hour
	case (score >= 30 || descendants >= 10) && age < 6*time.Hour:
		interval = 2 * time.Hour
	default:
		return nil
	}
	return &interval
}
