package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/sync"
)

const (
	blueskyAPI      = "https://public.api.bsky.app/xrpc"
	blueskyRate     = 30
	blueskyPageSize = 50

	QueueBlueskyHead = "sync_head:bluesky"
	QueueBluesky     = "sync:bluesky"
)

type blueskyCycleState struct {
	Query string
	Sort  string
	Limit int
}

type blueskySearchPostsResponse struct {
	Cursor string            `json:"cursor"`
	Posts  []blueskyPostView `json:"posts"`
}

type blueskyPostView struct {
	URI         string `json:"uri"`
	CID         string `json:"cid"`
	IndexedAt   string `json:"indexedAt"`
	ReplyCount  int    `json:"replyCount"`
	RepostCount int    `json:"repostCount"`
	LikeCount   int    `json:"likeCount"`
	QuoteCount  int    `json:"quoteCount"`
	Author      struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Record struct {
		Type      string   `json:"$type"`
		Text      string   `json:"text"`
		CreatedAt string   `json:"createdAt"`
		Langs     []string `json:"langs"`
		Embed     struct {
			Type string `json:"$type"`
		} `json:"embed"`
	} `json:"record"`
}

type BlueskySyncer struct {
	client  BlueskyClient
	limiter *ratelimit.Limiter
}

func NewBlueskySyncer(seen sync.SeenStore) *BlueskySyncer {
	return NewBlueskySyncerWithClient(newBlueskyHTTPClient(), seen)
}

func NewBlueskySyncerWithClient(client BlueskyClient, seen sync.SeenStore) *BlueskySyncer {
	if seen == nil {
		seen = sync.NewNoopSeenStore()
	}
	if client == nil {
		client = newBlueskyHTTPClient()
	}
	return &BlueskySyncer{
		client:  client,
		limiter: ratelimit.New(blueskyRate),
	}
}

func (b *BlueskySyncer) Provider() string  { return "bluesky" }
func (b *BlueskySyncer) HeadQueue() string { return QueueBlueskyHead }

func (b *BlueskySyncer) BuildJob(stream db.ProviderStream, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	query, _ := stream.Config["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("bluesky stream %q missing required config field: query", stream.Name)
	}

	state := newBlueskyCycleState(stream, query)
	cycleID := ""
	if cycle != nil {
		cycleID = cycle.ID
	}
	correlationID := uuid.NewString()
	payload, err := json.Marshal(db.SyncJob{
		CorrelationID:    correlationID,
		CycleID:          cycleID,
		ProviderStreamID: stream.ID,
		Provider:         "bluesky",
		JobType:          "search_posts",
		Payload:          state.payload(),
		Priority:         5,
		MaxAttempts:      3,
	})
	if err != nil {
		return nil, fmt.Errorf("bluesky BuildJob marshal: %w", err)
	}

	return &db.ScheduledJob{
		ID:            stream.ID,
		CorrelationID: correlationID,
		Type:          "sync:bluesky",
		Payload:       payload,
		Priority:      5,
		MaxRetry:      3,
		Timeout:       5 * time.Minute,
	}, nil
}

func (b *BlueskySyncer) Execute(ctx context.Context, job *db.SyncJob) (*sync.Result, error) {
	if err := b.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("bluesky rate limit wait: %w", err)
	}

	state := blueskyCycleStateFromPayload(job.Payload)
	log := slog.With(
		"component", "source_syncer",
		"provider", "bluesky",
		"provider_stream_id", job.ProviderStreamID,
		"cycle_id", job.CycleID,
		"correlation_id", job.CorrelationID,
		"query", state.Query,
		"sort", state.Sort,
		"limit", state.limit(),
	)
	log.Debug("bluesky: searching top posts")
	body, err := b.client.SearchPosts(ctx, state)
	if err != nil {
		return nil, err
	}
	log.Debug("bluesky: fetched search page", "provider_record_count", len(body.Posts), "has_next_cursor", body.Cursor != "")

	result := &sync.Result{
		ProviderStreamUpdates: map[string]any{},
		CursorUpdates:         map[string]any{},
	}
	var newestAcceptedURI string
	var newestAcceptedCreatedAt string
	for _, post := range body.Posts {
		if post.URI == "" {
			continue
		}
		if newestAcceptedURI == "" {
			newestAcceptedURI = post.URI
			newestAcceptedCreatedAt = post.Record.CreatedAt
		}

		result.Records = append(result.Records, &sync.RawRecord{
			ExternalID:        post.URI,
			SourceRevision:    0,
			Type:              "post",
			Label:             blueskyLabel(post),
			Content:           post.Record.Text,
			EnrichmentContent: blueskyEnrichmentContent(post),
			SourceURL:         blueskyPostURL(post),
			Metadata: map[string]any{
				"platform":      "bluesky",
				"query":         state.Query,
				"sort":          state.sort(),
				"uri":           post.URI,
				"cid":           post.CID,
				"author_did":    post.Author.DID,
				"author_handle": post.Author.Handle,
				"author_name":   post.Author.DisplayName,
				"created_at":    post.Record.CreatedAt,
				"indexed_at":    post.IndexedAt,
				"langs":         post.Record.Langs,
				"like_count":    post.LikeCount,
				"repost_count":  post.RepostCount,
				"reply_count":   post.ReplyCount,
				"quote_count":   post.QuoteCount,
				"embed_type":    post.Record.Embed.Type,
				"sample_limit":  state.limit(),
			},
		})
	}

	if newestAcceptedURI != "" {
		result.HeadBoundary = map[string]any{
			"uri":        newestAcceptedURI,
			"created_at": newestAcceptedCreatedAt,
		}
	}

	result.CompletionReason = sync.CompletionReasonExhausted

	log.Debug(
		"bluesky: execution result built",
		"accepted_record_count", len(result.Records),
		"has_more", result.HasMore,
		"completion_reason", result.CompletionReason,
		"provider_stream_update_keys", mapKeysForLog(result.ProviderStreamUpdates),
		"cursor_update_keys", mapKeysForLog(result.CursorUpdates),
		"head_boundary_keys", mapKeysForLog(result.HeadBoundary),
	)
	return result, nil
}

func newBlueskyCycleState(stream db.ProviderStream, query string) blueskyCycleState {
	sort, _ := stream.Config["sort"].(string)
	limit := intFromAny(stream.Config["limit"])
	return blueskyCycleState{
		Query: query,
		Sort:  sort,
		Limit: limit,
	}
}

func blueskyCycleStateFromPayload(payload map[string]any) blueskyCycleState {
	var state blueskyCycleState
	if payload == nil {
		return state
	}
	state.Query, _ = payload["query"].(string)
	state.Sort, _ = payload["sort"].(string)
	state.Limit = intFromAny(payload["limit"])
	return state
}

func (s blueskyCycleState) payload() map[string]any {
	return map[string]any{
		"query": s.Query,
		"sort":  s.sort(),
		"limit": s.limit(),
	}
}

func (s blueskyCycleState) sort() string {
	if strings.TrimSpace(s.Sort) == "" {
		return "top"
	}
	return strings.TrimSpace(s.Sort)
}

func (s blueskyCycleState) limit() int {
	if s.Limit <= 0 {
		return blueskyPageSize
	}
	if s.Limit > blueskyPageSize {
		return blueskyPageSize
	}
	return s.Limit
}

func blueskyLabel(post blueskyPostView) string {
	text := strings.Join(strings.Fields(post.Record.Text), " ")
	if text != "" {
		if len(text) > 80 {
			return text[:77] + "..."
		}
		return text
	}
	if post.Author.DisplayName != "" {
		return post.Author.DisplayName
	}
	if post.Author.Handle != "" {
		return post.Author.Handle
	}
	return post.URI
}

func blueskyPostURL(post blueskyPostView) string {
	if post.Author.Handle == "" {
		return ""
	}
	if !strings.HasPrefix(post.URI, "at://") {
		return ""
	}
	parts := strings.Split(post.URI, "/")
	if len(parts) != 5 || parts[3] != "app.bsky.feed.post" {
		return ""
	}
	rkey := parts[4]
	if rkey == "" {
		return ""
	}
	return "https://bsky.app/profile/" + post.Author.Handle + "/post/" + rkey
}

func blueskyEnrichmentContent(post blueskyPostView) string {
	parts := []string{
		"Bluesky post",
		"author: " + firstNonEmptyString(post.Author.DisplayName, post.Author.Handle),
		fmt.Sprintf("likes: %d", post.LikeCount),
		fmt.Sprintf("reposts: %d", post.RepostCount),
		fmt.Sprintf("replies: %d", post.ReplyCount),
	}
	if post.Record.Text != "" {
		parts = append(parts, "text: "+post.Record.Text)
	}
	return strings.Join(parts, "\n")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		var parsed int
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				return 0
			}
			parsed = parsed*10 + int(ch-'0')
		}
		return parsed
	default:
		return 0
	}
}
