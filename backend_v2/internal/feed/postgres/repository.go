package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"aladin/backend_v2/internal/feed"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresFeedRepository struct{ pool *pgxpool.Pool }

func NewFeedPostgres(pool *pgxpool.Pool) *PostgresFeedRepository {
	return &PostgresFeedRepository{pool: pool}
}

func (r *PostgresFeedRepository) List(ctx context.Context, params feed.FeedListParams) (map[string]any, error) {
	query := `
		SELECT
		    a.id, a.type, a.label, LEFT(COALESCE(a.content, ''), 500) AS content,
		    a.source_url, a.enrichment, a.metadata, a.user_status, a.created_at,
		    ps.provider AS source_type, ss.name AS source_name,
		    COALESCE(
		        LN(COALESCE((a.metadata->>'score')::float, 0) + 1) * 0.4
		        + GREATEST(0, 1 - EXTRACT(EPOCH FROM (now() - a.created_at)) / 2592000.0) * 0.6,
		        0
		    ) AS signal_score
		  FROM records a
		  JOIN LATERAL (
		      SELECT subscription_id
		        FROM tenant_item_matches
		       WHERE record_id = a.id
		         AND relevance_status = 'relevant'
		       ORDER BY updated_at DESC
		       LIMIT 1
		  ) tim ON TRUE
		  LEFT JOIN source_subscriptions ss ON ss.id = tim.subscription_id
		  LEFT JOIN provider_streams ps ON ps.id = ss.provider_stream_id
		 WHERE a.status != 'superseded'
		   AND a.user_status IS DISTINCT FROM 'dismissed'
	`
	args := []any{}
	argPos := 1
	if params.SourceType != "" {
		query += ` AND ps.provider = $` + strconv.Itoa(argPos)
		args = append(args, params.SourceType)
		argPos++
	}
	if params.Topic != "" {
		query += ` AND EXISTS (
		    SELECT 1 FROM jsonb_array_elements_text(a.enrichment->'topics') t WHERE t = $` + strconv.Itoa(argPos) + `)`
		args = append(args, params.Topic)
		argPos++
	}
	if params.SavedOnly {
		query += ` AND a.user_status = 'saved'`
	}
	if params.Sort == "signal" {
		query += ` ORDER BY signal_score DESC, a.created_at DESC`
	} else {
		query += ` ORDER BY a.created_at DESC`
	}
	query += ` LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]feed.FeedItem, 0)
	for rows.Next() {
		var item feed.FeedItem
		var enrichment, metadata []byte
		var createdAt time.Time
		item.Metadata = map[string]any{}
		if err := rows.Scan(
			&item.ID, &item.Type, &item.Label, &item.Content, &item.SourceURL,
			&enrichment, &metadata, &item.UserStatus, &createdAt,
			&item.SourceType, &item.SourceName, &item.SignalScore,
		); err != nil {
			return nil, err
		}
		if len(enrichment) > 0 {
			_ = json.Unmarshal(enrichment, &item.Enrichment)
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	countQuery := `SELECT COUNT(*) FROM records a JOIN LATERAL (SELECT subscription_id FROM tenant_item_matches WHERE record_id = a.id AND relevance_status = 'relevant' ORDER BY updated_at DESC LIMIT 1) tim ON TRUE LEFT JOIN source_subscriptions ss ON ss.id = tim.subscription_id LEFT JOIN provider_streams ps ON ps.id = ss.provider_stream_id WHERE a.status != 'superseded' AND a.user_status IS DISTINCT FROM 'dismissed'`
	countArgs := []any{}
	countPos := 1
	if params.SourceType != "" {
		countQuery += ` AND ps.provider = $` + strconv.Itoa(countPos)
		countArgs = append(countArgs, params.SourceType)
		countPos++
	}
	if params.Topic != "" {
		countQuery += ` AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(a.enrichment->'topics') t WHERE t = $` + strconv.Itoa(countPos) + `)`
		countArgs = append(countArgs, params.Topic)
		countPos++
	}
	if params.SavedOnly {
		countQuery += ` AND a.user_status = 'saved'`
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "limit": params.Limit, "offset": params.Offset}, nil
}

func (r *PostgresFeedRepository) Topics(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT jsonb_array_elements_text(enrichment->'topics') AS topic
		  FROM records a
		 WHERE a.enrichment IS NOT NULL
		   AND a.status != 'superseded'
		   AND EXISTS (
		       SELECT 1 FROM tenant_item_matches tim
		        WHERE tim.record_id = a.id
		          AND tim.relevance_status = 'relevant'
		   )
		 ORDER BY topic
		 LIMIT 100
	`)
	if err != nil {
		return []string{}, nil
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var topic string
		if rows.Scan(&topic) == nil {
			out = append(out, topic)
		}
	}
	return out, nil
}

func (r *PostgresFeedRepository) Sources(ctx context.Context) ([]feed.FeedSourceRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ss.id::text, ss.name, ps.provider, ss.status, ps.last_refresh_at
		  FROM source_subscriptions ss
		  JOIN provider_streams ps ON ps.id = ss.provider_stream_id
		 ORDER BY ss.name
	`)
	if err != nil {
		return []feed.FeedSourceRecord{}, nil
	}
	defer rows.Close()
	out := make([]feed.FeedSourceRecord, 0)
	for rows.Next() {
		var rec feed.FeedSourceRecord
		var lastSynced *time.Time
		if rows.Scan(&rec.ID, &rec.Name, &rec.Type, &rec.SyncState, &lastSynced) == nil {
			if lastSynced != nil {
				v := lastSynced.Format(time.RFC3339)
				rec.LastSyncedAt = &v
			}
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *PostgresFeedRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	var err error
	if status == "" {
		_, err = r.pool.Exec(ctx, `UPDATE records SET user_status = NULL WHERE id = $1`, id)
	} else {
		_, err = r.pool.Exec(ctx, `UPDATE records SET user_status = $2 WHERE id = $1`, id, status)
	}
	return err
}
