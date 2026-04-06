package syncers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

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

	s := NewBlueskySyncer(fakeSeenStore{known: map[string]bool{"at://did:plc:alice/app.bsky.feed.post/seen": true}})
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"cursor":"next-cursor","posts":[
			{"uri":"at://did:plc:alice/app.bsky.feed.post/new3","cid":"cid-3","indexedAt":"2026-04-03T12:00:00Z","author":{"did":"did:plc:alice","handle":"alice.bsky.social","displayName":"Alice"},"record":{"$type":"app.bsky.feed.post","text":"newest","createdAt":"2026-04-03T12:00:00Z","langs":["en"]},"replyCount":1,"repostCount":2,"likeCount":3,"quoteCount":4},
			{"uri":"at://did:plc:alice/app.bsky.feed.post/new2","cid":"cid-2","indexedAt":"2026-04-02T12:00:00Z","author":{"did":"did:plc:alice","handle":"alice.bsky.social","displayName":"Alice"},"record":{"$type":"app.bsky.feed.post","text":"newer","createdAt":"2026-04-02T12:00:00Z","langs":["en"]},"replyCount":0,"repostCount":1,"likeCount":2,"quoteCount":0},
			{"uri":"at://did:plc:alice/app.bsky.feed.post/seen","cid":"cid-1","indexedAt":"2026-04-01T12:00:00Z","author":{"did":"did:plc:alice","handle":"alice.bsky.social","displayName":"Alice"},"record":{"$type":"app.bsky.feed.post","text":"seen","createdAt":"2026-04-01T12:00:00Z","langs":["en"]},"replyCount":0,"repostCount":0,"likeCount":1,"quoteCount":0}
		]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

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

	s := NewBlueskySyncer(sync.NewNoopSeenStore())
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"cursor":"next-cursor","posts":[
			{"uri":"at://did:plc:alice/app.bsky.feed.post/new3","cid":"cid-3","indexedAt":"2026-04-03T12:00:00Z","author":{"did":"did:plc:alice","handle":"alice.bsky.social","displayName":"Alice"},"record":{"$type":"app.bsky.feed.post","text":"newest","createdAt":"2026-04-03T12:00:00Z","langs":["en"]},"replyCount":1,"repostCount":2,"likeCount":3,"quoteCount":4},
			{"uri":"at://did:plc:alice/app.bsky.feed.post/new2","cid":"cid-2","indexedAt":"2026-04-02T12:00:00Z","author":{"did":"did:plc:alice","handle":"alice.bsky.social","displayName":"Alice"},"record":{"$type":"app.bsky.feed.post","text":"newer","createdAt":"2026-04-02T12:00:00Z","langs":["en"]},"replyCount":0,"repostCount":1,"likeCount":2,"quoteCount":0}
		]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

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

	s := NewBlueskySyncer(fakeSeenStore{known: map[string]bool{"at://did:plc:alice/app.bsky.feed.post/seen": true}})
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"cursor":"next-cursor","posts":[
			{"uri":"at://did:plc:alice/app.bsky.feed.post/seen","cid":"cid-1","indexedAt":"2026-04-01T12:00:00Z","author":{"did":"did:plc:alice","handle":"alice.bsky.social","displayName":"Alice"},"record":{"$type":"app.bsky.feed.post","text":"seen","createdAt":"2026-04-01T12:00:00Z","langs":["en"]},"replyCount":0,"repostCount":0,"likeCount":1,"quoteCount":0}
		]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

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
