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
		SELECT COUNT(*) FROM records WHERE status IN ('pending', 'enriched', 'embedded')
	`).Scan(&pendingPipeline); err != nil {
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
			"enriched":  0,
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
