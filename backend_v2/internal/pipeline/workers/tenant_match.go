package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
)

type TenantMatchWorker struct {
	items       db.SourceItemRepository
	enrichments db.SourceItemEnrichmentRepository
	subs        db.SourceSubscriptionRepository
	matches     db.TenantItemMatchRepository
	enqueuer    pipeline.Enqueuer
	judge       llm.RelevanceJudge
}

func NewTenantMatchWorker(
	items db.SourceItemRepository,
	enrichments db.SourceItemEnrichmentRepository,
	subs db.SourceSubscriptionRepository,
	matches db.TenantItemMatchRepository,
	enqueuer pipeline.Enqueuer,
	judge llm.RelevanceJudge,
) *TenantMatchWorker {
	return &TenantMatchWorker{
		items:       items,
		enrichments: enrichments,
		subs:        subs,
		matches:     matches,
		enqueuer:    enqueuer,
		judge:       judge,
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
		"source_item_id", p.SourceItemID,
		"source_revision", p.SourceRevision,
		"correlation_id", p.CorrelationID,
	)

	item, err := w.items.Get(ctx, p.SourceItemID)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load source item: %w", err)})
	}
	enrichment, err := w.enrichments.Get(ctx, p.SourceItemID, p.SourceRevision)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load enrichment: %w", err)})
	}
	subs, err := w.subs.ListActiveByProviderStream(ctx, item.ProviderStreamID)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load subscriptions: %w", err)})
	}

	matched := 0
	for _, sub := range subs {
		if item.OwnerUserID != nil && *item.OwnerUserID != sub.UserID {
			continue
		}
		intentMatched := subscriptionIntentMatches(sub.Policy, item, enrichment)
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
				ItemTitle:        item.Title,
				ItemSummary:      enrichment.Summary,
				ItemEntities:     enrichment.Entities,
				ItemTopics:       enrichment.Topics,
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
			SourceItemID:    item.ID,
			SourceRevision:  item.SourceRevision,
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

		recordID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(sub.ID+":"+item.ID+":"+fmt.Sprint(item.SourceRevision))).String()
		recordPayload, err := json.Marshal(pipeline.RecordPayload{
			RecordID:       recordID,
			CorrelationID:  p.CorrelationID,
			KgID:           sub.KgID,
			SourceID:       sub.ID,
			ExternalID:     item.ExternalID,
			SourceRevision: item.SourceRevision,
			Type:           item.Type,
			Label:          item.Title,
			Content:        item.ContentExcerpt,
			SourceURL:      item.SourceURL,
			Metadata: map[string]any{
				"provider":            item.Provider,
				"provider_stream_id":  item.ProviderStreamID,
				"source_item_id":      item.ID,
				"source_subscription": sub.ID,
				"source_revision":     item.SourceRevision,
			},
			Summary:               enrichment.Summary,
			Entities:              enrichment.Entities,
			Topics:                enrichment.Topics,
			KeyClaims:             enrichment.KeyClaims,
			LowConfidenceEntities: enrichment.LowConfidenceEntities,
		})
		if err != nil {
			return tenantMatchErrResult(p, pipeline.ErrPermanent{Cause: fmt.Errorf("marshal tenant record payload: %w", err)})
		}
		if err := w.enqueuer.EnqueueStage(ctx, pipeline.TaskEmbed, recordID, recordPayload); err != nil {
			return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("enqueue tenant record: %w", err)})
		}

		match.RecordID = &recordID
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
		RecordID:      p.SourceItemID,
		CorrelationID: p.CorrelationID,
	}
}

func tenantMatchErrResult(p pipeline.TenantMatchPayload, err error) pipeline.Result {
	return pipeline.Result{
		TaskType:      pipeline.TaskTenantMatch,
		Err:           err,
		RecordID:      p.SourceItemID,
		CorrelationID: p.CorrelationID,
	}
}

func subscriptionIntentMatches(policy map[string]any, item *db.SourceItem, enrichment *db.SourceItemEnrichment) bool {
	if len(policy) == 0 || onlyLegacyPolicy(policy) {
		return true
	}
	haystack := strings.ToLower(strings.Join(append([]string{
		item.Title,
		item.ContentExcerpt,
		item.ContextExcerpt,
		enrichment.Summary,
	}, append(enrichment.Entities, enrichment.Topics...)...), " "))
	for _, key := range []string{"keywords", "topics", "entities", "domains"} {
		for _, token := range policyStrings(policy[key]) {
			if token != "" && strings.Contains(haystack, strings.ToLower(token)) {
				return true
			}
		}
	}
	return false
}

func onlyLegacyPolicy(policy map[string]any) bool {
	if len(policy) == 1 {
		_, ok := policy["legacy_source_id"]
		return ok
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
