package service

import "testing"

func TestBuildSourcePayloadBlueskySearchDefaults(t *testing.T) {
	t.Parallel()

	payload, err := buildSourcePayload(SourceCreateInput{
		Kind:  "bluesky_search",
		Query: "  AI   Agents  ",
	})
	if err != nil {
		t.Fatalf("buildSourcePayload error: %v", err)
	}
	if payload.Type != "bluesky" || payload.StreamKind != "search" {
		t.Fatalf("type/kind = %s/%s, want bluesky/search", payload.Type, payload.StreamKind)
	}
	if payload.StreamKey != "ai agents" {
		t.Fatalf("StreamKey = %q, want normalized key", payload.StreamKey)
	}
	if got := payload.Config["query"]; got != "AI Agents" {
		t.Fatalf("config.query = %v, want normalized display query", got)
	}
	if got := payload.Config["sort"]; got != "top" {
		t.Fatalf("config.sort = %v, want top", got)
	}
	if got := payload.Config["limit"]; got != 50 {
		t.Fatalf("config.limit = %v, want 50", got)
	}
	if got := payload.Config["sync_interval_seconds"]; got != 300 {
		t.Fatalf("config.sync_interval_seconds = %v, want 300", got)
	}
	keywords, ok := payload.Policy["keywords"].([]string)
	if !ok {
		t.Fatalf("policy.keywords = %#v, want []string", payload.Policy["keywords"])
	}
	if len(keywords) != 2 || keywords[0] != "ai agents" || keywords[1] != "agents" {
		t.Fatalf("policy.keywords = %#v, want query-derived keywords", keywords)
	}
}

func TestBuildSourcePayloadBlueskySearchClampsConfig(t *testing.T) {
	t.Parallel()

	limit := 500
	interval := 10
	payload, err := buildSourcePayload(SourceCreateInput{
		Kind:                "bluesky_search",
		Query:               "llm",
		Limit:               &limit,
		SyncIntervalSeconds: &interval,
	})
	if err != nil {
		t.Fatalf("buildSourcePayload error: %v", err)
	}
	if got := payload.Config["limit"]; got != 50 {
		t.Fatalf("config.limit = %v, want clamp to 50", got)
	}
	if got := payload.Config["sync_interval_seconds"]; got != 60 {
		t.Fatalf("config.sync_interval_seconds = %v, want min 60", got)
	}
}

func TestBuildSourcePayloadBlueskyAccountUnsupported(t *testing.T) {
	t.Parallel()

	_, err := buildSourcePayload(SourceCreateInput{
		Kind:   "bluesky_account",
		Handle: "alice.bsky.social",
	})
	if err == nil {
		t.Fatal("buildSourcePayload error = nil, want unsupported error")
	}
}
