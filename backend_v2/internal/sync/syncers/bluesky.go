package syncers

import (
	"context"
	"encoding/json"
	"fmt"
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
	blueskyPageSize = 100

	QueueBlueskyHead = "sync_head:bluesky"
	QueueBluesky     = "sync:bluesky"
)

type blueskyCycleState struct {
	Query                 string
	Cursor                string
	NextLastSeenURI       string
	NextLastSeenCreatedAt string
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
	seen    sync.SeenStore
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
		seen:    seen,
	}
}

func (b *BlueskySyncer) SourceType() string { return "bluesky" }
func (b *BlueskySyncer) HeadQueue() string  { return QueueBlueskyHead }

func (b *BlueskySyncer) BuildJob(source db.Source, cycle *db.SyncCycle) (*db.ScheduledJob, error) {
	query, _ := source.Config["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("bluesky source %q missing required config field: query", source.Name)
	}

	state := newBlueskyCycleState(source, cycle, query)
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
		SourceType:    "bluesky",
		JobType:       "search_posts",
		Payload:       state.payload(),
		Priority:      5,
		MaxAttempts:   3,
	})
	if err != nil {
		return nil, fmt.Errorf("bluesky BuildJob marshal: %w", err)
	}

	return &db.ScheduledJob{
		ID:            source.ID,
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
	body, err := b.client.SearchPosts(ctx, state)
	if err != nil {
		return nil, err
	}

	result := &sync.Result{
		SourceUpdates: map[string]any{},
		CursorUpdates: map[string]any{},
	}
	externalIDs := make([]string, 0, len(body.Posts))
	for _, post := range body.Posts {
		if post.URI != "" {
			externalIDs = append(externalIDs, post.URI)
		}
	}
	known, err := b.seen.Seen(ctx, job.SourceID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("bluesky seen lookup: %w", err)
	}
	hitBoundary := false
	var newestAcceptedURI string
	var newestAcceptedCreatedAt string
	for _, post := range body.Posts {
		if post.URI == "" {
			continue
		}

		if known[post.URI] {
			hitBoundary = true
			result.CompletionReason = sync.CompletionReasonSeenOverlap
			break
		}

		if state.NextLastSeenURI == "" {
			state.NextLastSeenURI = post.URI
			state.NextLastSeenCreatedAt = post.Record.CreatedAt
		}
		if newestAcceptedURI == "" {
			newestAcceptedURI = post.URI
			newestAcceptedCreatedAt = post.Record.CreatedAt
		}

		result.Records = append(result.Records, &sync.RawRecord{
			ExternalID: post.URI,
			Type:       "post",
			Label:      blueskyLabel(post),
			Content:    post.Record.Text,
			SourceURL:  blueskyPostURL(post),
			Metadata: map[string]any{
				"platform":      "bluesky",
				"query":         state.Query,
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
			},
		})
	}

	if newestAcceptedURI != "" {
		result.HeadBoundary = map[string]any{
			"uri":        newestAcceptedURI,
			"created_at": newestAcceptedCreatedAt,
		}
	}

	if state.NextLastSeenURI != "" {
		result.SourceUpdates["last_seen_post_uri"] = state.NextLastSeenURI
		result.SourceUpdates["last_seen_post_created_at"] = state.NextLastSeenCreatedAt
	}

	if !hitBoundary && body.Cursor != "" {
		result.HasMore = true
		result.CursorUpdates["cursor"] = body.Cursor
	} else {
		if result.CompletionReason == "" {
			result.CompletionReason = sync.CompletionReasonExhausted
		}
		result.CursorUpdates["cursor"] = ""
	}

	return result, nil
}

func newBlueskyCycleState(source db.Source, cycle *db.SyncCycle, query string) blueskyCycleState {
	state := blueskyCycleState{
		Query: query,
	}
	if cycle != nil && len(cycle.Cursor) > 0 {
		state = blueskyCycleStateFromPayload(cycle.Cursor)
		if state.Query == "" {
			state.Query = query
		}
		return state
	}
	return state
}

func blueskyCycleStateFromPayload(payload map[string]any) blueskyCycleState {
	var state blueskyCycleState
	if payload == nil {
		return state
	}
	state.Query, _ = payload["query"].(string)
	state.Cursor, _ = payload["cursor"].(string)
	state.NextLastSeenURI, _ = payload["next_last_seen_uri"].(string)
	state.NextLastSeenCreatedAt, _ = payload["next_last_seen_created_at"].(string)
	return state
}

func (s blueskyCycleState) payload() map[string]any {
	return map[string]any{
		"query":                     s.Query,
		"cursor":                    s.Cursor,
		"next_last_seen_uri":        s.NextLastSeenURI,
		"next_last_seen_created_at": s.NextLastSeenCreatedAt,
	}
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
