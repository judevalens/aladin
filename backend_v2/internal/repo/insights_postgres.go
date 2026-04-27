package repo

import (
	"context"
	"strconv"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresInsightRepository struct{ pool *pgxpool.Pool }

func NewInsightPostgres(pool *pgxpool.Pool) *PostgresInsightRepository {
	return &PostgresInsightRepository{pool: pool}
}

func (r *PostgresInsightRepository) List(ctx context.Context, params coreservice.InsightListParams) (map[string]any, error) {
	query := `
		SELECT id::text, type, title, body, entity, topic,
		       record_ids, confidence, user_status, created_at
		  FROM insights
		 WHERE 1=1
	`
	args := []any{}
	argPos := 1
	if params.Type != "" {
		query += ` AND type = $` + strconv.Itoa(argPos)
		args = append(args, params.Type)
		argPos++
	}
	if params.Status != "" {
		query += ` AND user_status = $` + strconv.Itoa(argPos)
		args = append(args, params.Status)
		argPos++
	}
	query += ` ORDER BY confidence DESC, created_at DESC LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []coreservice.InsightRecord
	for rows.Next() {
		var item coreservice.InsightRecord
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Body, &item.Entity, &item.Topic, &item.RecordIDs, &item.Confidence, &item.UserStatus, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	countQuery := `SELECT COUNT(*) FROM insights WHERE 1=1`
	countArgs := []any{}
	countPos := 1
	if params.Type != "" {
		countQuery += ` AND type = $` + strconv.Itoa(countPos)
		countArgs = append(countArgs, params.Type)
		countPos++
	}
	if params.Status != "" {
		countQuery += ` AND user_status = $` + strconv.Itoa(countPos)
		countArgs = append(countArgs, params.Status)
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "limit": params.Limit, "offset": params.Offset}, nil
}

func (r *PostgresInsightRepository) Stats(ctx context.Context) (map[string]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT type, user_status, COUNT(*) AS count
		  FROM insights
		 GROUP BY type, user_status
	`)
	if err != nil {
		return map[string]any{"byType": map[string]int{}, "byStatus": map[string]int{}}, nil
	}
	defer rows.Close()
	byType := map[string]int{}
	byStatus := map[string]int{}
	for rows.Next() {
		var t, status string
		var count int
		if rows.Scan(&t, &status, &count) == nil {
			byType[t] += count
			byStatus[status] += count
		}
	}
	return map[string]any{"byType": byType, "byStatus": byStatus}, nil
}

func (r *PostgresInsightRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE insights SET user_status = $2 WHERE id = $1::uuid`, id, status)
	return err
}
