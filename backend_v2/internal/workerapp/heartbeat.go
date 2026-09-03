package workerapp

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

func writeHeartbeat(ctx context.Context, pool *pgxpool.Pool, inspector *asynq.Inspector) {
	payload, _ := json.Marshal(collectQueueStats(inspector))
	if _, err := pool.Exec(ctx, `UPDATE worker_heartbeat SET updated_at = now(), stats = $1::jsonb WHERE id = 1`, payload); err != nil {
		slog.Warn("worker heartbeat write failed", "component", "worker", "err", err)
	}
}

// collectQueueStats aggregates Asynq queue counts across all queues for the worker heartbeat.
func collectQueueStats(inspector *asynq.Inspector) map[string]any {
	queues, err := inspector.Queues()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var pending, active, scheduled, retry, archived, completed, processed, failed int
	for _, queue := range queues {
		info, err := inspector.GetQueueInfo(queue)
		if err != nil {
			continue
		}
		pending += info.Pending
		active += info.Active
		scheduled += info.Scheduled
		retry += info.Retry
		archived += info.Archived
		completed += info.Completed
		processed += info.Processed
		failed += info.Failed
	}
	return map[string]any{
		"queues": len(queues), "pending": pending, "active": active, "scheduled": scheduled,
		"retry": retry, "archived": archived, "completed": completed, "processed": processed, "failed": failed,
	}
}
