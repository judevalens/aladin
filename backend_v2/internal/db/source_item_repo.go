package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSourceItemRepo struct{ pool *pgxpool.Pool }

func NewSourceItemRepository(pool *pgxpool.Pool) SourceItemRepository {
	return &pgSourceItemRepo{pool}
}

func (r *pgSourceItemRepo) Upsert(ctx context.Context, item *SourceItem) (*SourceItemUpsertResult, error) {
	providerMeta, err := json.Marshal(item.ProviderMetadata)
	if err != nil {
		return nil, fmt.Errorf("SourceItem Upsert marshal provider metadata: %w", err)
	}
	hydrationMeta, err := json.Marshal(item.HydrationMetadata)
	if err != nil {
		return nil, fmt.Errorf("SourceItem Upsert marshal hydration metadata: %w", err)
	}
	var owner any
	if item.OwnerUserID != nil {
		owner = *item.OwnerUserID
	}

	row := r.pool.QueryRow(ctx, `
		WITH incoming AS (
			SELECT
				$1::text AS id,
				$2::uuid AS provider_stream_id,
				$3::text AS provider,
				$4::text AS external_id,
				$5::uuid AS owner_user_id,
				$6::text AS item_type,
				$7::text AS title,
				$8::text AS canonical_url,
				$9::text AS source_url,
				$10::text AS content_excerpt,
				$11::text AS context_excerpt,
				$12::bigint AS source_revision,
				$13::jsonb AS provider_metadata,
				$14::jsonb AS hydration_metadata
		),
		upserted AS (
			INSERT INTO source_items (
				id, provider_stream_id, provider, external_id, owner_user_id,
				item_type, title, canonical_url, source_url, content_excerpt,
				context_excerpt, source_revision, provider_metadata, hydration_metadata,
				fetched_at, hydrated_at, updated_at
			)
			SELECT
				id, provider_stream_id, provider, external_id, owner_user_id,
				item_type, title, canonical_url, source_url, content_excerpt,
				context_excerpt, source_revision, provider_metadata, hydration_metadata,
				now(),
				CASE WHEN hydration_metadata <> '{}'::jsonb THEN now() ELSE NULL END,
				now()
			FROM incoming
			ON CONFLICT (id) DO UPDATE
			SET provider_stream_id = EXCLUDED.provider_stream_id,
			    item_type = EXCLUDED.item_type,
			    title = EXCLUDED.title,
			    canonical_url = EXCLUDED.canonical_url,
			    source_url = EXCLUDED.source_url,
			    content_excerpt = EXCLUDED.content_excerpt,
			    context_excerpt = EXCLUDED.context_excerpt,
			    source_revision = EXCLUDED.source_revision,
			    provider_metadata = EXCLUDED.provider_metadata,
			    hydration_metadata = EXCLUDED.hydration_metadata,
			    hydrated_at = CASE WHEN EXCLUDED.hydration_metadata <> '{}'::jsonb THEN now() ELSE source_items.hydrated_at END,
			    updated_at = now()
			WHERE source_items.source_revision < EXCLUDED.source_revision
			RETURNING source_items.*, (xmax = 0) AS inserted
		)
		SELECT
			id, provider_stream_id::text, provider, external_id, owner_user_id::text,
			item_type, title, COALESCE(canonical_url, ''), COALESCE(source_url, ''),
			content_excerpt, context_excerpt, source_revision, provider_metadata,
			hydration_metadata, fetched_at, hydrated_at, inserted, true AS changed, false AS stale
		FROM upserted
		UNION ALL
		SELECT
			existing.id, existing.provider_stream_id::text, existing.provider, existing.external_id,
			existing.owner_user_id::text, existing.item_type, existing.title,
			COALESCE(existing.canonical_url, ''), COALESCE(existing.source_url, ''),
			existing.content_excerpt, existing.context_excerpt, existing.source_revision,
			existing.provider_metadata, existing.hydration_metadata, existing.fetched_at,
			existing.hydrated_at, false AS inserted, false AS changed,
			(existing.source_revision >= incoming.source_revision) AS stale
		FROM source_items existing, incoming
		WHERE existing.id = incoming.id
		  AND NOT EXISTS (SELECT 1 FROM upserted)
		LIMIT 1
	`, item.ID, item.ProviderStreamID, item.Provider, item.ExternalID, owner, item.Type, item.Title,
		item.CanonicalURL, item.SourceURL, item.ContentExcerpt, item.ContextExcerpt, item.SourceRevision,
		string(providerMeta), string(hydrationMeta))

	out, err := scanSourceItemUpsert(row)
	if err != nil {
		return nil, fmt.Errorf("SourceItem Upsert: %w", err)
	}
	return out, nil
}

func (r *pgSourceItemRepo) Get(ctx context.Context, id string) (*SourceItem, error) {
	var providerMeta, hydrationMeta []byte
	item := &SourceItem{}
	var owner *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider_stream_id::text, provider, external_id, owner_user_id::text,
		       item_type, title, COALESCE(canonical_url, ''), COALESCE(source_url, ''),
		       content_excerpt, context_excerpt, source_revision, provider_metadata,
		       hydration_metadata, fetched_at, hydrated_at
		  FROM source_items
		 WHERE id = $1
	`, id).Scan(&item.ID, &item.ProviderStreamID, &item.Provider, &item.ExternalID, &owner,
		&item.Type, &item.Title, &item.CanonicalURL, &item.SourceURL, &item.ContentExcerpt,
		&item.ContextExcerpt, &item.SourceRevision, &providerMeta, &hydrationMeta,
		&item.FetchedAt, &item.HydratedAt)
	if err != nil {
		return nil, fmt.Errorf("SourceItem Get: %w", err)
	}
	item.OwnerUserID = owner
	_ = json.Unmarshal(providerMeta, &item.ProviderMetadata)
	_ = json.Unmarshal(hydrationMeta, &item.HydrationMetadata)
	return item, nil
}

func scanSourceItemUpsert(row pgx.Row) (*SourceItemUpsertResult, error) {
	item := &SourceItem{}
	var providerMeta, hydrationMeta []byte
	var owner *string
	var inserted, changed, stale bool
	err := row.Scan(&item.ID, &item.ProviderStreamID, &item.Provider, &item.ExternalID, &owner,
		&item.Type, &item.Title, &item.CanonicalURL, &item.SourceURL, &item.ContentExcerpt,
		&item.ContextExcerpt, &item.SourceRevision, &providerMeta, &hydrationMeta,
		&item.FetchedAt, &item.HydratedAt, &inserted, &changed, &stale)
	if err != nil {
		return nil, err
	}
	item.OwnerUserID = owner
	_ = json.Unmarshal(providerMeta, &item.ProviderMetadata)
	_ = json.Unmarshal(hydrationMeta, &item.HydrationMetadata)
	return &SourceItemUpsertResult{Item: item, Changed: changed, IsStale: stale, Inserted: inserted}, nil
}

type pgSourceItemEnrichmentRepo struct{ pool *pgxpool.Pool }

func NewSourceItemEnrichmentRepository(pool *pgxpool.Pool) SourceItemEnrichmentRepository {
	return &pgSourceItemEnrichmentRepo{pool}
}

func (r *pgSourceItemEnrichmentRepo) Save(ctx context.Context, e *SourceItemEnrichment) error {
	entities, _ := json.Marshal(e.Entities)
	topics, _ := json.Marshal(e.Topics)
	keyClaims, _ := json.Marshal(e.KeyClaims)
	lowConfidence, _ := json.Marshal(e.LowConfidenceEntities)
	if e.Status == "" {
		e.Status = "ready"
	}
	var embedding any
	if len(e.Embedding) > 0 {
		embedding = vectorToString(e.Embedding)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO source_item_enrichments (
			source_item_id, source_revision, summary, entities, topics, key_claims,
			low_confidence_entities, embedding, status
		)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::vector, $9)
		ON CONFLICT (source_item_id, source_revision) DO UPDATE
		SET summary = EXCLUDED.summary,
		    entities = EXCLUDED.entities,
		    topics = EXCLUDED.topics,
		    key_claims = EXCLUDED.key_claims,
		    low_confidence_entities = EXCLUDED.low_confidence_entities,
		    embedding = EXCLUDED.embedding,
		    status = EXCLUDED.status
	`, e.SourceItemID, e.SourceRevision, e.Summary, string(entities), string(topics), string(keyClaims),
		string(lowConfidence), embedding, e.Status)
	if err != nil {
		return fmt.Errorf("SourceItemEnrichment Save: %w", err)
	}
	return nil
}

func (r *pgSourceItemEnrichmentRepo) Get(ctx context.Context, sourceItemID string, sourceRevision int64) (*SourceItemEnrichment, error) {
	e := &SourceItemEnrichment{SourceItemID: sourceItemID, SourceRevision: sourceRevision}
	var entities, topics, keyClaims, lowConfidence []byte
	err := r.pool.QueryRow(ctx, `
		SELECT summary, entities, topics, key_claims, low_confidence_entities, status
		  FROM source_item_enrichments
		 WHERE source_item_id = $1 AND source_revision = $2
	`, sourceItemID, sourceRevision).Scan(&e.Summary, &entities, &topics, &keyClaims, &lowConfidence, &e.Status)
	if err != nil {
		return nil, fmt.Errorf("SourceItemEnrichment Get: %w", err)
	}
	_ = json.Unmarshal(entities, &e.Entities)
	_ = json.Unmarshal(topics, &e.Topics)
	_ = json.Unmarshal(keyClaims, &e.KeyClaims)
	_ = json.Unmarshal(lowConfidence, &e.LowConfidenceEntities)
	return e, nil
}

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
			subscription_id, source_item_id, source_revision, match_source,
			overlap_entities, relevance_status, relevance_score, relevance_reason, record_id
		)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, $8, $9)
		ON CONFLICT (subscription_id, source_item_id, source_revision) DO UPDATE
		SET match_source = EXCLUDED.match_source,
		    overlap_entities = EXCLUDED.overlap_entities,
		    relevance_status = EXCLUDED.relevance_status,
		    relevance_score = EXCLUDED.relevance_score,
		    relevance_reason = EXCLUDED.relevance_reason,
		    record_id = COALESCE(EXCLUDED.record_id, tenant_item_matches.record_id),
		    updated_at = now()
	`, m.SubscriptionID, m.SourceItemID, m.SourceRevision, m.MatchSource, string(overlap),
		m.RelevanceStatus, m.RelevanceScore, m.RelevanceReason, m.RecordID)
	if err != nil {
		return fmt.Errorf("TenantItemMatch Save: %w", err)
	}
	return nil
}

func (r *pgTenantItemMatchRepo) AttachRecord(ctx context.Context, subscriptionID string, sourceItemID string, sourceRevision int64, recordID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tenant_item_matches
		SET record_id = $4,
		    updated_at = now()
		WHERE subscription_id = $1::uuid
		  AND source_item_id = $2
		  AND source_revision = $3
	`, subscriptionID, sourceItemID, sourceRevision, recordID)
	if err != nil {
		return fmt.Errorf("TenantItemMatch AttachRecord: %w", err)
	}
	return nil
}
