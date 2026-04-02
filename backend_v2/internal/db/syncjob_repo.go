package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSyncJobRepo struct{ pool *pgxpool.Pool }

func NewSyncJobRepository(pool *pgxpool.Pool) SyncJobRepository {
	return &pgSyncJobRepo{pool}
}

func (r *pgSyncJobRepo) Enqueue(ctx context.Context, job *SyncJob) error {
	payload, _ := json.Marshal(job.Payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sync_jobs
			(id, source_id, snapshot_id, source_type, job_type, payload,
			 priority, scheduled_at, attempts, max_attempts, status)
		VALUES
			(gen_random_uuid(), $1::uuid, $2::uuid, $3, $4,
			 $5::jsonb, $6, now(), 0, $7, 'pending')
	`, job.SourceID, job.SnapshotID, job.SourceType, job.JobType,
		string(payload), job.Priority, job.MaxAttempts)
	return err
}

func (r *pgSyncJobRepo) DequeueOne(ctx context.Context, sourceType string) (*SyncJob, error) {
	var job SyncJob
	var payloadJSON []byte

	err := r.pool.QueryRow(ctx, `
		UPDATE sync_jobs
		SET status = 'processing'
		WHERE id = (
			SELECT id FROM sync_jobs
			WHERE status = 'pending'
			  AND source_type = $1
			  AND scheduled_at <= now()
			ORDER BY priority DESC, scheduled_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id::text, source_id::text, snapshot_id::text,
		          source_type, job_type, payload,
		          priority, attempts, max_attempts
	`, sourceType).Scan(
		&job.ID, &job.SourceID, &job.SnapshotID,
		&job.SourceType, &job.JobType, &payloadJSON,
		&job.Priority, &job.Attempts, &job.MaxAttempts,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("DequeueOne: %w", err)
	}
	_ = json.Unmarshal(payloadJSON, &job.Payload)
	return &job, nil
}

func (r *pgSyncJobRepo) MarkDead(ctx context.Context, id, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sync_jobs SET status = 'dead', last_error = $1 WHERE id = $2::uuid`,
		errMsg, id)
	return err
}

func (r *pgSyncJobRepo) MarkPendingWithBackoff(ctx context.Context, id, errMsg string, attempts, backoffSeconds int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sync_jobs
		SET status = 'pending', attempts = $1, last_error = $2,
		    scheduled_at = now() + $3 * interval '1 second'
		WHERE id = $4::uuid
	`, attempts, errMsg, backoffSeconds, id)
	return err
}

func (r *pgSyncJobRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sync_jobs WHERE id = $1::uuid`, id)
	return err
}
