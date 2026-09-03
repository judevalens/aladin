package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// workerHeartbeatStaleAfter — a heartbeat older than this means the worker is down or wedged.
// The worker beats every 30s, so 2 min tolerates a few missed beats.
const workerHeartbeatStaleAfter = 2 * time.Minute

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

	// Real Asynq queue counts + liveness from the worker heartbeat (the api is Redis-free, so the
	// worker writes these). A stale/absent heartbeat means the worker is down or wedged.
	queue := map[string]any{
		"pendingCycles":   activeCycles,
		"pendingPipeline": pendingPipeline,
	}
	workerUp := false
	var lastHeartbeat any
	var hbAt time.Time
	var statsRaw []byte
	switch err := r.pool.QueryRow(ctx, `SELECT updated_at, stats FROM worker_heartbeat WHERE id = 1`).Scan(&hbAt, &statsRaw); {
	case err == nil:
		workerUp = time.Since(hbAt) < workerHeartbeatStaleAfter
		lastHeartbeat = hbAt.UTC().Format(time.RFC3339)
		var asynqStats map[string]any
		if json.Unmarshal(statsRaw, &asynqStats) == nil {
			for k, v := range asynqStats {
				queue[k] = v // active/pending/retry/archived/failed/processed/…
			}
		}
	case err == pgx.ErrNoRows:
		// heartbeat table present but never written — worker hasn't started
	default:
		return nil, err
	}

	return map[string]any{
		"workerUp":      workerUp,
		"lastHeartbeat": lastHeartbeat,
		"pipeline": map[string]any{
			"enriched": enrichedRecords,
		},
		"queue": queue,
	}, nil
}

// PipelineStats is a read-only ingestion health snapshot. It only SELECTs — it never
// touches the write/retry path — so it surfaces stuck records (the silent-failure problem)
// safely. Terminal statuses are 'complete' (enriched + entities + embedded) and
// 'failed'; everything else is in-flight. "Stuck" = in-flight with no stage progress for
// 15+ minutes.
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

	var inFlight, stuck, failed, completed1h, completed24h, relationships int
	var oldestInFlightSecs, avgLatencySecs *float64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status NOT IN ('complete', 'failed')`).Scan(&inFlight); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status NOT IN ('complete', 'failed') AND updated_at < now() - interval '15 minutes'`).Scan(&stuck); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status = 'failed'`).Scan(&failed); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status = 'complete' AND updated_at >= now() - interval '1 hour'`).Scan(&completed1h); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM records WHERE status = 'complete' AND updated_at >= now() - interval '24 hours'`).Scan(&completed24h); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM relationships`).Scan(&relationships); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT extract(epoch FROM (now() - min(updated_at))) FROM records WHERE status NOT IN ('complete', 'failed')`).Scan(&oldestInFlightSecs); err != nil {
		return nil, err
	}
	// End-to-end latency (capture → complete) over records completed in the last 24h.
	if err := r.pool.QueryRow(ctx, `SELECT avg(extract(epoch FROM (updated_at - created_at))) FROM records WHERE status = 'complete' AND updated_at >= now() - interval '24 hours'`).Scan(&avgLatencySecs); err != nil {
		return nil, err
	}

	return map[string]any{
		"records": map[string]any{
			"byStatus":           recordsByStatus,
			"inFlight":           inFlight,
			"stuckOver15Min":     stuck,
			"failed":             failed,
			"oldestInFlightSecs": oldestInFlightSecs, // null when nothing is in-flight
		},
		"throughput": map[string]any{
			"completedLast1h":  completed1h,
			"completedLast24h": completed24h,
			"avgLatencySecs":   avgLatencySecs, // null when none completed recently
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
