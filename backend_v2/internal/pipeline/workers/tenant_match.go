package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline"
)

type TenantMatchWorker struct {
	records  db.RecordRepository
	streams  db.ProviderStreamRepository
	subs     db.SourceSubscriptionRepository
	matches  db.TenantItemMatchRepository
	deciders map[string]RecordRelevanceDecider
}

func NewTenantMatchWorker(
	records db.RecordRepository,
	streams db.ProviderStreamRepository,
	subs db.SourceSubscriptionRepository,
	matches db.TenantItemMatchRepository,
	deciders ...RecordRelevanceDecider,
) *TenantMatchWorker {
	if len(deciders) == 0 {
		deciders = []RecordRelevanceDecider{NewPolicyRelevanceDecider()}
	}
	byKey := make(map[string]RecordRelevanceDecider, len(deciders))
	for _, decider := range deciders {
		if decider != nil {
			byKey[decider.Key()] = decider
		}
	}
	return &TenantMatchWorker{
		records:  records,
		streams:  streams,
		subs:     subs,
		matches:  matches,
		deciders: byKey,
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
	stream, err := w.streams.GetByID(ctx, record.ProviderStreamID)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load provider stream: %w", err)})
	}
	subs, err := w.subs.ListActiveByProviderStream(ctx, record.ProviderStreamID)
	if err != nil {
		return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("load subscriptions: %w", err)})
	}

	matched := 0
	triggersByKG := make(map[string]*pipeline.InsightTrigger)
	for _, sub := range subs {
		if record.OwnerUserID != nil && *record.OwnerUserID != sub.UserID {
			continue
		}
		input := BuildRecordRelevanceInput(record, sub, stream)
		decision, ok, err := w.decide(ctx, input, log)
		if err != nil {
			return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("decide relevance: %w", err)})
		}
		if !ok {
			continue
		}
		match := &db.TenantItemMatch{
			SubscriptionID:  sub.ID,
			RecordID:        record.ID,
			SourceRevision:  record.SourceRevision,
			MatchSource:     decision.MatchSource,
			RelevanceStatus: string(decision.Status),
			RelevanceScore:  &decision.Score,
			RelevanceReason: decision.Reason,
		}
		if err := w.matches.Save(ctx, match); err != nil {
			return tenantMatchErrResult(p, pipeline.ErrTransient{Cause: fmt.Errorf("save tenant match: %w", err)})
		}
		if decision.Status != RelevanceRelevant {
			continue
		}
		matched++
		trigger := triggersByKG[sub.KgID]
		if trigger == nil {
			trigger = &pipeline.InsightTrigger{
				KgID:           sub.KgID,
				RecordID:       record.ID,
				SourceRevision: record.SourceRevision,
				CorrelationID:  p.CorrelationID,
			}
			triggersByKG[sub.KgID] = trigger
		}
		trigger.SubscriptionIDs = appendUnique(trigger.SubscriptionIDs, sub.ID)
		trigger.GeneratorKeys = appendUnique(trigger.GeneratorKeys, input.Policy.InsightGenerators...)
	}

	log.Info("tenant_match: complete", "subscription_count", len(subs), "matched_count", matched)
	p.InsightTriggers = make([]pipeline.InsightTrigger, 0, len(triggersByKG))
	for _, trigger := range triggersByKG {
		p.InsightTriggers = append(p.InsightTriggers, *trigger)
	}
	payload, _ := json.Marshal(p)
	return pipeline.Result{
		Type:          pipeline.ResultTenantMatchDone,
		TaskType:      pipeline.TaskTenantMatch,
		Payload:       payload,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
}

func (w *TenantMatchWorker) decide(ctx context.Context, input RecordRelevanceInput, log *slog.Logger) (RecordRelevanceDecision, bool, error) {
	keys := input.Policy.Deciders
	if len(keys) == 0 {
		keys = []string{DefaultRelevanceDeciderKey}
	}
	for _, key := range keys {
		decider := w.deciders[key]
		if decider == nil {
			log.Warn("tenant_match: unknown relevance decider", "decider", key)
			continue
		}
		decision, err := decider.Decide(ctx, input)
		if err != nil {
			return RecordRelevanceDecision{}, false, err
		}
		switch decision.Status {
		case RelevanceRelevant, RelevanceNotRelevant:
			if decision.MatchSource == "" {
				decision.MatchSource = decider.Key()
			}
			return decision, true, nil
		case RelevanceNoDecision:
			continue
		default:
			log.Warn("tenant_match: invalid relevance decision status", "decider", key, "status", decision.Status)
		}
	}
	return RecordRelevanceDecision{}, false, nil
}

func tenantMatchErrResult(p pipeline.TenantMatchPayload, err error) pipeline.Result {
	return pipeline.Result{
		TaskType:      pipeline.TaskTenantMatch,
		Err:           err,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
}

func appendUnique(values []string, next ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(next))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range next {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		values = append(values, value)
		seen[value] = struct{}{}
	}
	return values
}
