package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSystemRepository struct{ pool *pgxpool.Pool }

func NewSystemPostgres(pool *pgxpool.Pool) *PostgresSystemRepository {
	return &PostgresSystemRepository{pool: pool}
}

func (r *PostgresSystemRepository) Ready(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *PostgresSystemRepository) WorkerStatus(ctx context.Context) (map[string]any, error) {
	var pendingPipeline int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM records WHERE status IN ('pending', 'captured')
	`).Scan(&pendingPipeline); err != nil {
		return nil, err
	}
	var enrichedRecords int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM records WHERE status = 'enriched'
	`).Scan(&enrichedRecords); err != nil {
		return nil, err
	}
	var activeCycles int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sync_cycles WHERE status IN ('active', 'running')
	`).Scan(&activeCycles); err != nil {
		return nil, err
	}
	return map[string]any{
		"pipeline": map[string]any{
			"enriched":  enrichedRecords,
			"embedded":  0,
			"promoted":  0,
			"errors":    0,
			"last_tick": nil,
		},
		"queue": map[string]any{
			"pending":         activeCycles,
			"pendingPipeline": pendingPipeline,
			"deadJobs":        0,
		},
	}, nil
}

// PipelineStats is a read-only ingestion health snapshot. It only SELECTs — it
// never touches the write/retry path — so it surfaces stuck records (the silent-
// failure problem) safely. "Stuck" = a record that hasn't reached 'enriched' an
// hour after capture.
func (r *PostgresSystemRepository) PipelineStats(ctx context.Context) (map[string]any, error) {
	recordsByStatus, err := r.countBy(ctx, `SELECT status, count(*) FROM records GROUP BY status`)
	if err != nil {
		return nil, err
	}
	insightsByType, err := r.countBy(ctx, `SELECT type, count(*) FROM insights GROUP BY type`)
	if err != nil {
		return nil, err
	}
	insightsByStatus, err := r.countBy(ctx, `SELECT user_status, count(*) FROM insights GROUP BY user_status`)
	if err != nil {
		return nil, err
	}
	matchesByStatus, err := r.countBy(ctx, `SELECT relevance_status, count(*) FROM tenant_item_matches GROUP BY relevance_status`)
	if err != nil {
		return nil, err
	}

	var stuck, enriched24h, relationships int
	var oldestPendingSecs *float64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status <> 'enriched' AND created_at < now() - interval '1 hour'`).Scan(&stuck); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status = 'enriched' AND created_at >= now() - interval '24 hours'`).Scan(&enriched24h); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM relationships`).Scan(&relationships); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT extract(epoch FROM (now() - min(created_at))) FROM records WHERE status <> 'enriched'`).Scan(&oldestPendingSecs); err != nil {
		return nil, err
	}

	return map[string]any{
		"records": map[string]any{
			"byStatus":          recordsByStatus,
			"stuckOverOneHour":  stuck,
			"enrichedLast24h":   enriched24h,
			"oldestPendingSecs": oldestPendingSecs, // null when nothing is pending
		},
		"insights": map[string]any{
			"byType":   insightsByType,
			"byStatus": insightsByStatus,
		},
		"matches":       map[string]any{"byStatus": matchesByStatus},
		"relationships": relationships,
	}, nil
}

// countBy runs a `SELECT key, count(*) ... GROUP BY key` and returns key→count
// (a NULL key collapses to "unknown").
func (r *PostgresSystemRepository) countBy(ctx context.Context, query string) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key *string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		k := "unknown"
		if key != nil {
			k = *key
		}
		out[k] = count
	}
	return out, rows.Err()
}
