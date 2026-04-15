package syncers

import (
	"context"
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

type fakeBlueskyClient struct {
	resp *blueskySearchPostsResponse
	err  error
}

type blueskyFixturePost struct {
	uri       string
	cid       string
	indexedAt string
	text      string
	createdAt string
}

func (f fakeBlueskyClient) SearchPosts(ctx context.Context, state blueskyCycleState) (*blueskySearchPostsResponse, error) {
	return f.resp, f.err
}

func blueskyResponse(posts ...blueskyFixturePost) *blueskySearchPostsResponse {
	resp := &blueskySearchPostsResponse{}
	for _, p := range posts {
		post := blueskyPostView{
			URI:       p.uri,
			CID:       p.cid,
			IndexedAt: p.indexedAt,
		}
		post.Author.DID = "did:plc:alice"
		post.Author.Handle = "alice.bsky.social"
		post.Author.DisplayName = "Alice"
		post.Record.Type = "app.bsky.feed.post"
		post.Record.Text = p.text
		post.Record.CreatedAt = p.createdAt
		post.Record.Langs = []string{"en"}
		resp.Posts = append(resp.Posts, post)
	}
	return resp
}

func TestBlueskyBuildJobBootstrapForNewSource(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.Source{
		ID:   "source-1",
		KgID: "kg-1",
		Name: "llm search",
		Type: "bluesky",
		Config: map[string]any{
			"query": "llm agents",
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
	if got := payload.Payload["query"]; got != "llm agents" {
		t.Fatalf("query = %v, want llm agents", got)
	}
}

func TestBlueskyBuildJobCarriesLastSeenBoundary(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.Source{
		ID:   "source-1",
		KgID: "kg-1",
		Name: "llm search",
		Type: "bluesky",
		Config: map[string]any{
			"query":                     "llm agents",
			"last_seen_post_uri":        "at://did:plc:alice/app.bsky.feed.post/seen",
			"last_seen_post_created_at": "2026-04-01T12:00:00Z",
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload.Payload["stop_before_uri"]; ok {
		t.Fatal("stop_before_uri should not be present")
	}
	if _, ok := payload.Payload["bootstrap"]; ok {
		t.Fatal("bootstrap should not be present")
	}
}

func TestBlueskyExecuteStopsAtSeenID(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncerWithClient(fakeBlueskyClient{resp: func() *blueskySearchPostsResponse {
		resp := blueskyResponse(
			blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/new3", "cid-3", "2026-04-03T12:00:00Z", "newest", "2026-04-03T12:00:00Z"},
			blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/new2", "cid-2", "2026-04-02T12:00:00Z", "newer", "2026-04-02T12:00:00Z"},
			blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/seen", "cid-1", "2026-04-01T12:00:00Z", "seen", "2026-04-01T12:00:00Z"},
		)
		resp.Cursor = "next-cursor"
		return resp
	}()}, fakeSeenStore{known: map[string]bool{"at://did:plc:alice/app.bsky.feed.post/seen": true}})

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "bluesky",
		Payload: map[string]any{
			"query":                     "llm agents",
			"cursor":                    "",
			"next_last_seen_uri":        "",
			"next_last_seen_created_at": "",
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
	if len(result.Artifacts) != 2 {
		t.Fatalf("artifact count = %d, want 2", len(result.Artifacts))
	}
	if got := result.SourceUpdates["last_seen_post_uri"]; got != "at://did:plc:alice/app.bsky.feed.post/new3" {
		t.Fatalf("last_seen_post_uri = %v, want newest uri", got)
	}
	if got := result.HeadBoundary["uri"]; got != "at://did:plc:alice/app.bsky.feed.post/new3" {
		t.Fatalf("head_boundary.uri = %v, want newest uri", got)
	}
	if got := result.HeadBoundary["created_at"]; got != "2026-04-03T12:00:00Z" {
		t.Fatalf("head_boundary.created_at = %v, want newest timestamp", got)
	}
}

func TestBlueskyExecuteCarriesNextBoundaryAcrossPagination(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncerWithClient(fakeBlueskyClient{resp: func() *blueskySearchPostsResponse {
		resp := blueskyResponse(
			blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/new3", "cid-3", "2026-04-03T12:00:00Z", "newest", "2026-04-03T12:00:00Z"},
			blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/new2", "cid-2", "2026-04-02T12:00:00Z", "newer", "2026-04-02T12:00:00Z"},
		)
		resp.Cursor = "next-cursor"
		return resp
	}()}, sync.NewNoopSeenStore())

	job := &db.SyncJob{
		CorrelationID: "corr-1",
		SourceID:      "source-1",
		SourceType:    "bluesky",
		Priority:      5,
		MaxAttempts:   3,
		Payload: map[string]any{
			"query":                     "llm agents",
			"cursor":                    "",
			"next_last_seen_uri":        "",
			"next_last_seen_created_at": "",
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
	if got := result.CursorUpdates["cursor"]; got != "next-cursor" {
		t.Fatalf("cursor = %v, want next-cursor", got)
	}
	if got := result.SourceUpdates["last_seen_post_uri"]; got != "at://did:plc:alice/app.bsky.feed.post/new3" {
		t.Fatalf("last_seen_post_uri = %v, want newest uri", got)
	}
	if got := result.HeadBoundary["uri"]; got != "at://did:plc:alice/app.bsky.feed.post/new3" {
		t.Fatalf("head_boundary.uri = %v, want newest uri", got)
	}
}

func TestBlueskyExecuteDoesNotAdvanceHighWaterMarkWhenFirstPostIsSeen(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncerWithClient(fakeBlueskyClient{resp: func() *blueskySearchPostsResponse {
		resp := blueskyResponse(
			blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/seen", "cid-1", "2026-04-01T12:00:00Z", "seen", "2026-04-01T12:00:00Z"},
		)
		resp.Cursor = "next-cursor"
		return resp
	}()}, fakeSeenStore{known: map[string]bool{"at://did:plc:alice/app.bsky.feed.post/seen": true}})

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "bluesky",
		Payload: map[string]any{
			"query":                     "llm agents",
			"cursor":                    "",
			"next_last_seen_uri":        "",
			"next_last_seen_created_at": "",
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("artifact count = %d, want 0", len(result.Artifacts))
	}
	if result.CompletionReason != sync.CompletionReasonSeenOverlap {
		t.Fatalf("CompletionReason = %q, want %q", result.CompletionReason, sync.CompletionReasonSeenOverlap)
	}
	if _, ok := result.SourceUpdates["last_seen_post_uri"]; ok {
		t.Fatal("last_seen_post_uri should not advance when boundary is first post")
	}
	if len(result.HeadBoundary) != 0 {
		t.Fatalf("head_boundary = %v, want empty", result.HeadBoundary)
	}
}
