package workers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"aladin/backend_v2/internal/db"
)

const DefaultRelevanceDeciderKey = "subscription_intent_keywords"

type RecordRelevanceInput struct {
	Record RecordRelevanceRecord
	Target RelevanceTarget
	Policy RelevancePolicy
}

type RecordRelevanceRecord struct {
	ID             string
	Type           string
	Provider       string
	ExternalID     string
	Title          string
	SourceURL      string
	ContentExcerpt string
	ContextExcerpt string
	SourceRevision int64
	Summary        string
	Entities       []string
	Topics         []string
	KeyClaims      []string
}

type RelevanceTarget struct {
	KgID             string
	UserID           string
	SubscriptionID   string
	ProviderStreamID string
}

type RelevancePolicy struct {
	Keywords          []string
	Topics            []string
	Entities          []string
	Domains           []string
	Deciders          []string
	InsightGenerators []string
}

type RelevanceDecisionStatus string

const (
	RelevanceNoDecision  RelevanceDecisionStatus = "no_decision"
	RelevanceRelevant    RelevanceDecisionStatus = "relevant"
	RelevanceNotRelevant RelevanceDecisionStatus = "not_relevant"
)

type RecordRelevanceDecision struct {
	Status      RelevanceDecisionStatus
	Score       float64
	Reason      string
	MatchSource string
}

type RecordRelevanceDecider interface {
	Key() string
	Decide(ctx context.Context, input RecordRelevanceInput) (RecordRelevanceDecision, error)
}

type PolicyRelevanceDecider struct{}

func NewPolicyRelevanceDecider() PolicyRelevanceDecider {
	return PolicyRelevanceDecider{}
}

func (PolicyRelevanceDecider) Key() string { return DefaultRelevanceDeciderKey }

func (d PolicyRelevanceDecider) Decide(ctx context.Context, input RecordRelevanceInput) (RecordRelevanceDecision, error) {
	terms := input.Policy.matchTerms()
	if len(terms) == 0 {
		return RecordRelevanceDecision{
			Status:      RelevanceRelevant,
			Score:       0.5,
			Reason:      "no relevance constraints configured",
			MatchSource: d.Key(),
		}, nil
	}

	haystack := input.Record.searchText()
	for _, term := range terms {
		if strings.Contains(haystack, strings.ToLower(term)) {
			return RecordRelevanceDecision{
				Status:      RelevanceRelevant,
				Score:       0.65,
				Reason:      fmt.Sprintf("matched relevance policy term %q", term),
				MatchSource: d.Key(),
			}, nil
		}
	}
	return RecordRelevanceDecision{Status: RelevanceNoDecision}, nil
}

func BuildRecordRelevanceInput(record *db.Record, sub *db.SourceSubscription, stream *db.ProviderStream) RecordRelevanceInput {
	policy := mergeRelevancePolicy(streamRelevancePolicy(stream), mapRelevancePolicy(sub.Policy))
	return RecordRelevanceInput{
		Record: RecordRelevanceRecord{
			ID:             record.ID,
			Type:           record.Type,
			Provider:       record.Provider,
			ExternalID:     record.ExternalID,
			Title:          record.Title,
			SourceURL:      record.SourceURL,
			ContentExcerpt: record.ContentExcerpt,
			ContextExcerpt: record.ContextExcerpt,
			SourceRevision: record.SourceRevision,
			Summary:        record.Enrichment.Summary,
			Entities:       record.Enrichment.Entities,
			Topics:         record.Enrichment.Topics,
			KeyClaims:      record.Enrichment.KeyClaims,
		},
		Target: RelevanceTarget{
			KgID:             sub.KgID,
			UserID:           sub.UserID,
			SubscriptionID:   sub.ID,
			ProviderStreamID: sub.ProviderStreamID,
		},
		Policy: policy,
	}
}

func streamRelevancePolicy(stream *db.ProviderStream) RelevancePolicy {
	if stream == nil {
		return RelevancePolicy{}
	}
	if raw, ok := stream.Config["relevance_policy"]; ok {
		if policy, ok := raw.(map[string]any); ok {
			return mapRelevancePolicy(policy)
		}
	}
	return mapRelevancePolicy(stream.Config)
}

func mapRelevancePolicy(raw map[string]any) RelevancePolicy {
	if len(raw) == 0 {
		return RelevancePolicy{}
	}
	return RelevancePolicy{
		Keywords:          policyStrings(raw["keywords"]),
		Topics:            policyStrings(raw["topics"]),
		Entities:          policyStrings(raw["entities"]),
		Domains:           policyStrings(raw["domains"]),
		Deciders:          policyStrings(raw["relevance_deciders"]),
		InsightGenerators: policyStrings(raw["insight_generators"]),
	}
}

func mergeRelevancePolicy(base RelevancePolicy, override RelevancePolicy) RelevancePolicy {
	out := base
	if override.Keywords != nil {
		out.Keywords = override.Keywords
	}
	if override.Topics != nil {
		out.Topics = override.Topics
	}
	if override.Entities != nil {
		out.Entities = override.Entities
	}
	if override.Domains != nil {
		out.Domains = override.Domains
	}
	if override.Deciders != nil {
		out.Deciders = override.Deciders
	}
	if override.InsightGenerators != nil {
		out.InsightGenerators = override.InsightGenerators
	}
	return out
}

func (p RelevancePolicy) matchTerms() []string {
	terms := make([]string, 0, len(p.Keywords)+len(p.Topics)+len(p.Entities)+len(p.Domains))
	terms = append(terms, p.Keywords...)
	terms = append(terms, p.Topics...)
	terms = append(terms, p.Entities...)
	terms = append(terms, p.Domains...)
	return uniqueNonEmpty(terms)
}

func (r RecordRelevanceRecord) searchText() string {
	parts := []string{
		r.Title,
		r.SourceURL,
		sourceDomain(r.SourceURL),
		r.ContentExcerpt,
		r.ContextExcerpt,
		r.Summary,
		strings.Join(r.Entities, " "),
		strings.Join(r.Topics, " "),
		strings.Join(r.KeyClaims, " "),
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func policyStrings(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	case []string:
		return uniqueNonEmpty(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return uniqueNonEmpty(out)
	default:
		return nil
	}
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func sourceDomain(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
