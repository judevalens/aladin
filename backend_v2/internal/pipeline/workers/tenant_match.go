package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
)

type TenantMatchWorker struct {
	records db.RecordRepository
	subs    db.SourceSubscriptionRepository
	matches db.TenantItemMatchRepository
	judge   llm.RelevanceJudge
}

func NewTenantMatchWorker(
	records db.RecordRepository,
	subs db.SourceSubscriptionRepository,
	matches db.TenantItemMatchRepository,
	judge llm.RelevanceJudge,
) *TenantMatchWorker {
	return &TenantMatchWorker{
		records: records,
		subs:    subs,
		matches: matches,
		judge:   judge,
	}
}

func (w *TenantMatchWorker) TaskType() string { return pipeline.TaskTenantMatch }
func (w *TenantMatchWorker) Concurrency() int { return 10 }

func (w *TenantMatchWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.TenantMatchPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskTenantMatch,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("unmarshal tenant match payload: %w", err)},
		}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "tenant_match",
		"record_id", p.RecordID,
		"source_revision", p.SourceRevision,
		"correlation_id", p.CorrelationID,
	)

	record, err := w.records.Get(ctx, p.RecordID)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load record: %w", err)})
	}
	subs, err := w.subs.ListActiveByProviderStream(ctx, record.ProviderStreamID)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load subscriptions: %w", err)})
	}

	matched := 0
	for _, sub := range subs {
		if record.OwnerUserID != nil && *record.OwnerUserID != sub.UserID {
			continue
		}
		intentMatched := subscriptionIntentMatches(sub.Policy, record)
		// KG overlap should be resolved by a graph-backed collaborator later. Keep the
		// match-source contract explicit so this worker does not pretend it queried KG.
		if !intentMatched {
			continue
		}
		relevance := &llm.RelevanceResult{
			Relevant:   true,
			Confidence: 0.65,
			Reason:     "matched active source subscription intent",
		}
		if w.judge != nil {
			var err error
			relevance, err = w.judge.JudgeRelevance(ctx, llm.RelevanceInput{
				SubscriptionName: sub.Name,
				Policy:           sub.Policy,
				ItemTitle:        record.Title,
				ItemSummary:      record.Enrichment.Summary,
				ItemEntities:     record.Enrichment.Entities,
				ItemTopics:       record.Enrichment.Topics,
			})
			if err != nil {
				return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("judge relevance: %w", err)})
			}
		}

		status := "not_relevant"
		if relevance.Relevant {
			status = "relevant"
		}
		match := &db.TenantItemMatch{
			SubscriptionID:  sub.ID,
			RecordID:        record.ID,
			SourceRevision:  record.SourceRevision,
			MatchSource:     "intent",
			RelevanceStatus: status,
			RelevanceScore:  &relevance.Confidence,
			RelevanceReason: relevance.Reason,
		}
		if !relevance.Relevant {
			if err := w.matches.Save(ctx, match); err != nil {
				return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("save tenant match: %w", err)})
			}
			continue
		}

		if err := w.matches.Save(ctx, match); err != nil {
			return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("save tenant match: %w", err)})
		}
		matched++
	}

	log.Info("tenant_match: complete", "subscription_count", len(subs), "matched_count", matched)
	payload, _ := json.Marshal(p)
	return pipeline.Result{
		Type:          pipeline.ResultTenantMatchDone,
		TaskType:      pipeline.TaskTenantMatch,
		Payload:       payload,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
}

func tenantMatchErrResult(p pipeline.TenantMatchPayload, err error) pipeline.Result {
	return pipeline.Result{
		TaskType:      pipeline.TaskTenantMatch,
		Err:           err,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
}

func subscriptionIntentMatches(policy map[string]any, record *db.Record) bool {
	if len(policy) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join(append([]string{
		record.Title,
		record.ContentExcerpt,
		record.ContextExcerpt,
		record.Enrichment.Summary,
	}, append(record.Enrichment.Entities, record.Enrichment.Topics...)...), " "))
	for _, key := range []string{"keywords", "topics", "entities", "domains"} {
		for _, token := range policyStrings(policy[key]) {
			if token != "" && strings.Contains(haystack, strings.ToLower(token)) {
				return true
			}
		}
	}
	return false
}

func policyStrings(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
