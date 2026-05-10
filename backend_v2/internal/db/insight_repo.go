package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgInsightRepo struct{ pool *pgxpool.Pool }

func NewInsightRepository(pool *pgxpool.Pool) InsightRepository {
	return &pgInsightRepo{pool}
}

func (r *pgInsightRepo) ExistsRecent(ctx context.Context, kgID, insightType, key, title string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM insights
			WHERE kg_id = $1::uuid
			  AND type = $2
			  AND (entity = $3 OR topic = $3 OR title = $4)
			  AND created_at >= now() - interval '3 days'
		)
	`, kgID, insightType, key, title).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("ExistsRecent: %w", err)
	}
	return exists, nil
}

func (r *pgInsightRepo) Store(ctx context.Context, kgID string, insight *Insight) error {
	recordIDs, _ := json.Marshal(insight.RecordIDs)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO insights
			(kg_id, type, title, body, record_ids, entity, topic, confidence, user_status)
		VALUES
			($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, $8, 'pending')
	`, kgID, insight.Type, insight.Title, insight.Body,
		string(recordIDs), nullStr(insight.Entity), nullStr(insight.Topic), insight.Confidence)
	return err
}

type pgKnowledgeGraphRepo struct{ pool *pgxpool.Pool }

func NewKnowledgeGraphRepository(pool *pgxpool.Pool) KnowledgeGraphRepository {
	return &pgKnowledgeGraphRepo{pool}
}

func (r *pgKnowledgeGraphRepo) GetIDsWithEnrichedRecords(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ss.kg_id::text
		FROM records a
		JOIN tenant_item_matches tim ON tim.record_id = a.id
		JOIN source_subscriptions ss ON ss.id = tim.subscription_id
		WHERE a.enrichment IS NOT NULL
		  AND a.status NOT IN ('superseded', 'dismissed')
	`)
	if err != nil {
		return nil, fmt.Errorf("GetIDsWithEnrichedRecords: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
