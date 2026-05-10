package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSourceSubscriptionRepo struct{ pool *pgxpool.Pool }

func NewSourceSubscriptionRepository(pool *pgxpool.Pool) SourceSubscriptionRepository {
	return &pgSourceSubscriptionRepo{pool}
}

func (r *pgSourceSubscriptionRepo) ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*SourceSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, user_id::text, kg_id::text, provider_stream_id::text,
		       name, visibility, policy, status
		  FROM source_subscriptions
		 WHERE provider_stream_id = $1::uuid
		   AND status = 'active'
	`, providerStreamID)
	if err != nil {
		return nil, fmt.Errorf("SourceSubscription ListActiveByProviderStream: %w", err)
	}
	defer rows.Close()
	var subs []*SourceSubscription
	for rows.Next() {
		sub := &SourceSubscription{}
		var policy []byte
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.KgID, &sub.ProviderStreamID, &sub.Name, &sub.Visibility, &policy, &sub.Status); err != nil {
			return nil, fmt.Errorf("SourceSubscription scan: %w", err)
		}
		_ = json.Unmarshal(policy, &sub.Policy)
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (r *pgSourceSubscriptionRepo) Ensure(ctx context.Context, sub *SourceSubscription) (*SourceSubscription, error) {
	policy, err := json.Marshal(sub.Policy)
	if err != nil {
		return nil, fmt.Errorf("SourceSubscription Ensure marshal policy: %w", err)
	}
	out := &SourceSubscription{}
	var policyJSON []byte
	err = r.pool.QueryRow(ctx, `
		INSERT INTO source_subscriptions (user_id, kg_id, provider_stream_id, name, visibility, policy, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, COALESCE(NULLIF($5, ''), 'public'), $6::jsonb, COALESCE(NULLIF($7, ''), 'active'))
		ON CONFLICT (user_id, kg_id, provider_stream_id) DO UPDATE
		SET name = EXCLUDED.name,
		    policy = source_subscriptions.policy || EXCLUDED.policy,
		    status = EXCLUDED.status,
		    updated_at = now()
		RETURNING id::text, user_id::text, kg_id::text, provider_stream_id::text,
		          name, visibility, policy, status
	`, sub.UserID, sub.KgID, sub.ProviderStreamID, sub.Name, sub.Visibility, string(policy), sub.Status).
		Scan(&out.ID, &out.UserID, &out.KgID, &out.ProviderStreamID, &out.Name, &out.Visibility, &policyJSON, &out.Status)
	if err != nil {
		return nil, fmt.Errorf("SourceSubscription Ensure: %w", err)
	}
	_ = json.Unmarshal(policyJSON, &out.Policy)
	return out, nil
}

type pgTenantItemMatchRepo struct{ pool *pgxpool.Pool }

func NewTenantItemMatchRepository(pool *pgxpool.Pool) TenantItemMatchRepository {
	return &pgTenantItemMatchRepo{pool}
}

func (r *pgTenantItemMatchRepo) Save(ctx context.Context, m *TenantItemMatch) error {
	overlap, _ := json.Marshal(m.OverlapEntities)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenant_item_matches (
			subscription_id, record_id, source_revision, match_source,
			overlap_entities, relevance_status, relevance_score, relevance_reason
		)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, $8)
		ON CONFLICT (subscription_id, record_id, source_revision) DO UPDATE
		SET match_source = EXCLUDED.match_source,
		    overlap_entities = EXCLUDED.overlap_entities,
		    relevance_status = EXCLUDED.relevance_status,
		    relevance_score = EXCLUDED.relevance_score,
		    relevance_reason = EXCLUDED.relevance_reason,
		    updated_at = now()
	`, m.SubscriptionID, m.RecordID, m.SourceRevision, m.MatchSource, string(overlap),
		m.RelevanceStatus, m.RelevanceScore, m.RelevanceReason)
	if err != nil {
		return fmt.Errorf("TenantItemMatch Save: %w", err)
	}
	return nil
}
