package workers

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
)

func TestBuildRecordRelevanceInputMergesPolicy(t *testing.T) {
	t.Parallel()

	input := BuildRecordRelevanceInput(
		&db.Record{
			ID:               "record-1",
			Provider:         "bluesky",
			Type:             "post",
			Title:            "AI agents",
			ContentExcerpt:   "content",
			ProviderStreamID: "stream-1",
			Enrichment: db.RecordEnrichment{
				Topics: []string{"runtime"},
			},
		},
		&db.SourceSubscription{
			ID:               "sub-1",
			UserID:           "user-1",
			KgID:             "kg-1",
			ProviderStreamID: "stream-1",
			Policy: map[string]any{
				"keywords":           []any{"agents"},
				"insight_generators": []any{"topic_trend"},
			},
		},
		&db.ProviderStream{
			ID: "stream-1",
			Config: map[string]any{
				"relevance_policy": map[string]any{
					"keywords":           []any{"fallback"},
					"relevance_deciders": []any{DefaultRelevanceDeciderKey},
				},
			},
		},
	)

	if input.Target.KgID != "kg-1" || input.Target.SubscriptionID != "sub-1" {
		t.Fatalf("target = %+v", input.Target)
	}
	if len(input.Policy.Keywords) != 1 || input.Policy.Keywords[0] != "agents" {
		t.Fatalf("keywords = %#v, want subscription override", input.Policy.Keywords)
	}
	if len(input.Policy.Deciders) != 1 || input.Policy.Deciders[0] != DefaultRelevanceDeciderKey {
		t.Fatalf("deciders = %#v, want provider default", input.Policy.Deciders)
	}
	if len(input.Policy.InsightGenerators) != 1 || input.Policy.InsightGenerators[0] != "topic_trend" {
		t.Fatalf("insight generators = %#v", input.Policy.InsightGenerators)
	}
}

func TestPolicyRelevanceDeciderRelevantAndNoDecision(t *testing.T) {
	t.Parallel()

	decider := NewPolicyRelevanceDecider()
	relevant, err := decider.Decide(context.Background(), RecordRelevanceInput{
		Record: RecordRelevanceRecord{
			Title:   "Building AI agents",
			Summary: "agent runtime notes",
		},
		Policy: RelevancePolicy{Keywords: []string{"agents"}},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if relevant.Status != RelevanceRelevant {
		t.Fatalf("status = %q, want relevant", relevant.Status)
	}

	noDecision, err := decider.Decide(context.Background(), RecordRelevanceInput{
		Record: RecordRelevanceRecord{Title: "gardening"},
		Policy: RelevancePolicy{Keywords: []string{"agents"}},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if noDecision.Status != RelevanceNoDecision {
		t.Fatalf("status = %q, want no decision", noDecision.Status)
	}
}
