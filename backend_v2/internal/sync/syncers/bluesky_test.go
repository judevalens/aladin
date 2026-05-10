package syncers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

type fakeBlueskyClient struct {
	resp   *blueskySearchPostsResponse
	err    error
	states []blueskyCycleState
}

type blueskyFixturePost struct {
	uri       string
	cid       string
	indexedAt string
	text      string
	createdAt string
}

func (f *fakeBlueskyClient) SearchPosts(ctx context.Context, state blueskyCycleState) (*blueskySearchPostsResponse, error) {
	f.states = append(f.states, state)
	return f.resp, f.err
}

func blueskyResponse(posts ...blueskyFixturePost) *blueskySearchPostsResponse {
	resp := &blueskySearchPostsResponse{Cursor: "ignored-cursor"}
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

func TestBlueskyBuildJobUsesProviderStreamTopSample(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncer(nil)
	job, err := s.BuildJob(db.ProviderStream{
		ID:       "stream-1",
		Name:     "llm search",
		Provider: "bluesky",
		Config: map[string]any{
			"query": "llm agents",
			"limit": 100,
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ProviderStreamID != "stream-1" {
		t.Fatalf("ProviderStreamID = %q, want stream-1", payload.ProviderStreamID)
	}
	if got := payload.Payload["query"]; got != "llm agents" {
		t.Fatalf("query = %v, want llm agents", got)
	}
	if got := payload.Payload["sort"]; got != "top" {
		t.Fatalf("sort = %v, want top", got)
	}
	if got := payload.Payload["limit"]; got != float64(50) {
		t.Fatalf("limit = %v, want 50", got)
	}
	if _, ok := payload.Payload["cursor"]; ok {
		t.Fatal("cursor should not be present for top sampling")
	}
}

func TestBlueskyExecuteFetchesSingleTopPage(t *testing.T) {
	t.Parallel()

	client := &fakeBlueskyClient{resp: blueskyResponse(
		blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/new3", "cid-3", "2026-04-03T12:00:00Z", "newest", "2026-04-03T12:00:00Z"},
		blueskyFixturePost{"at://did:plc:alice/app.bsky.feed.post/new2", "cid-2", "2026-04-02T12:00:00Z", "newer", "2026-04-02T12:00:00Z"},
	)}
	s := NewBlueskySyncerWithClient(client, nil)

	result, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "stream-1",
		Provider:         "bluesky",
		Payload: map[string]any{
			"query": "llm agents",
			"sort":  "top",
			"limit": 50,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(client.states) != 1 {
		t.Fatalf("SearchPosts calls = %d, want 1", len(client.states))
	}
	if client.states[0].Sort != "top" || client.states[0].Limit != 50 {
		t.Fatalf("state = %#v, want top/50", client.states[0])
	}
	if result.HasMore {
		t.Fatal("HasMore = true, want false for one-shot top sample")
	}
	if result.CompletionReason != sync.CompletionReasonExhausted {
		t.Fatalf("CompletionReason = %q, want %q", result.CompletionReason, sync.CompletionReasonExhausted)
	}
	if len(result.CursorUpdates) != 0 {
		t.Fatalf("CursorUpdates = %v, want empty", result.CursorUpdates)
	}
	if len(result.ProviderStreamUpdates) != 0 {
		t.Fatalf("ProviderStreamUpdates = %v, want empty", result.ProviderStreamUpdates)
	}
	if len(result.NewCycles) != 0 {
		t.Fatalf("NewCycles = %d, want 0", len(result.NewCycles))
	}
	if len(result.Records) != 2 {
		t.Fatalf("record count = %d, want 2", len(result.Records))
	}
}

func TestBlueskyExecuteEmitsURIAndProvenance(t *testing.T) {
	t.Parallel()

	s := NewBlueskySyncerWithClient(&fakeBlueskyClient{resp: blueskyResponse(
		blueskyFixturePost{
			uri:       "at://did:plc:alice/app.bsky.feed.post/abc123",
			cid:       "cid-abc123",
			indexedAt: "2026-04-03T12:01:00Z",
			text:      "LLM agents for research",
			createdAt: "2026-04-03T12:00:00Z",
		},
	)}, nil)

	result, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "stream-1",
		Provider:         "bluesky",
		Payload: map[string]any{
			"query": "llm agents",
			"sort":  "top",
			"limit": 50,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(result.Records))
	}

	record := result.Records[0]
	if record.ExternalID != "at://did:plc:alice/app.bsky.feed.post/abc123" {
		t.Fatalf("ExternalID = %q, want Bluesky URI", record.ExternalID)
	}
	if record.SourceRevision != 0 {
		t.Fatalf("SourceRevision = %d, want stable v1 revision 0", record.SourceRevision)
	}
	if record.SourceURL != "https://bsky.app/profile/alice.bsky.social/post/abc123" {
		t.Fatalf("SourceURL = %q, want bsky post URL", record.SourceURL)
	}
	if !strings.Contains(record.EnrichmentContent, "LLM agents for research") {
		t.Fatalf("EnrichmentContent = %q, want post text", record.EnrichmentContent)
	}
	if got := record.Metadata["platform"]; got != "bluesky" {
		t.Fatalf("metadata.platform = %v, want bluesky", got)
	}
	if got := record.Metadata["query"]; got != "llm agents" {
		t.Fatalf("metadata.query = %v, want query", got)
	}
	if got := record.Metadata["sort"]; got != "top" {
		t.Fatalf("metadata.sort = %v, want top", got)
	}
	if got := record.Metadata["sample_limit"]; got != 50 {
		t.Fatalf("metadata.sample_limit = %v, want 50", got)
	}
	if got := record.Metadata["uri"]; got != record.ExternalID {
		t.Fatalf("metadata.uri = %v, want external ID", got)
	}
	if got := record.Metadata["cid"]; got != "cid-abc123" {
		t.Fatalf("metadata.cid = %v, want cid", got)
	}
	if got := record.Metadata["author_handle"]; got != "alice.bsky.social" {
		t.Fatalf("metadata.author_handle = %v, want alice.bsky.social", got)
	}
	if got := record.Metadata["created_at"]; got != "2026-04-03T12:00:00Z" {
		t.Fatalf("metadata.created_at = %v, want post created timestamp", got)
	}
}
