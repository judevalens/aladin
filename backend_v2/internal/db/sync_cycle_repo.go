package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSyncCycleRepo struct{ pool *pgxpool.Pool }

func NewSyncCycleRepository(pool *pgxpool.Pool) SyncCycleRepository {
	return &pgSyncCycleRepo{pool: pool}
}

func (r *pgSyncCycleRepo) ListActiveByProviderStream(ctx context.Context, providerStreamID string) ([]*SyncCycle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, COALESCE(provider_stream_id::text, ''), kind, status, cursor, head_boundary,
		       last_hydrated_at, COALESCE(target_kind, ''), COALESCE(target_external_id, ''),
		       COALESCE(completion_reason, ''), last_picked_at, created_at, completed_at
		  FROM sync_cycles
		 WHERE provider_stream_id = $1::uuid
		   AND status IN ('active', 'running')
		 ORDER BY COALESCE(last_hydrated_at, to_timestamp(0)) ASC, created_at ASC
	`, providerStreamID)
	if err != nil {
		return nil, fmt.Errorf("ListActiveByProviderStream: %w", err)
	}
	defer rows.Close()

	var cycles []*SyncCycle
	for rows.Next() {
		cycle := &SyncCycle{}
		var cursorJSON []byte
		var headJSON []byte
		if err := rows.Scan(
			&cycle.ID,
			&cycle.ProviderStreamID,
			&cycle.Kind,
			&cycle.Status,
			&cursorJSON,
			&headJSON,
			&cycle.LastHydratedAt,
			&cycle.TargetKind,
			&cycle.TargetExternalID,
			&cycle.CompletionReason,
			&cycle.LastPickedAt,
			&cycle.CreatedAt,
			&cycle.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("ListActiveByProviderStream scan: %w", err)
		}
		_ = json.Unmarshal(cursorJSON, &cycle.Cursor)
		_ = json.Unmarshal(headJSON, &cycle.HeadBoundary)
		cycles = append(cycles, cycle)
	}
	return cycles, rows.Err()
}

func (r *pgSyncCycleRepo) Create(ctx context.Context, cycle *SyncCycle) error {
	if cycle == nil {
		return fmt.Errorf("Create: nil cycle")
	}
	cursor, err := json.Marshal(nonNilMap(cycle.Cursor))
	if err != nil {
		return fmt.Errorf("Create marshal cursor: %w", err)
	}
	head, err := json.Marshal(nonNilMap(cycle.HeadBoundary))
	if err != nil {
		return fmt.Errorf("Create marshal head_boundary: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO sync_cycles (
			id, provider_stream_id, kind, status, cursor, head_boundary, last_hydrated_at,
			target_kind, target_external_id, completion_reason
		)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9, NULLIF($10, ''))
		ON CONFLICT DO NOTHING
	`, cycle.ID, cycle.ProviderStreamID, cycle.Kind, cycle.Status, cursor, head, cycle.LastHydratedAt, cycle.TargetKind, cycle.TargetExternalID, cycle.CompletionReason)
	if err != nil {
		return fmt.Errorf("Create: %w", err)
	}
	return nil
}

func (r *pgSyncCycleRepo) MarkRunning(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'running',
		    last_picked_at = now()
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("MarkRunning: %w", err)
	}
	return nil
}

func (r *pgSyncCycleRepo) UpdateProgress(ctx context.Context, id string, cursor map[string]any, headBoundary map[string]any, lastHydratedAt *time.Time) error {
	cursorPayload, err := json.Marshal(nonNilMap(cursor))
	if err != nil {
		return fmt.Errorf("UpdateProgress marshal cursor: %w", err)
	}
	headPayload, err := json.Marshal(nonNilMap(headBoundary))
	if err != nil {
		return fmt.Errorf("UpdateProgress marshal head_boundary: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'active',
		    cursor = $2::jsonb,
		    last_hydrated_at = COALESCE($4, last_hydrated_at),
		    head_boundary = CASE
		        WHEN head_boundary = '{}'::jsonb AND $3::jsonb <> '{}'::jsonb THEN $3::jsonb
		        ELSE head_boundary
		    END
		WHERE id = $1::uuid
	`, id, cursorPayload, headPayload, lastHydratedAt)
	if err != nil {
		return fmt.Errorf("UpdateProgress: %w", err)
	}
	return nil
}

func (r *pgSyncCycleRepo) MarkActive(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'active'
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("MarkActive: %w", err)
	}
	return nil
}

func (r *pgSyncCycleRepo) MarkFailed(ctx context.Context, id string, completionReason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'failed',
		    completion_reason = NULLIF($2, ''),
		    completed_at = now()
		WHERE id = $1::uuid
	`, id, completionReason)
	if err != nil {
		return fmt.Errorf("MarkFailed: %w", err)
	}
	return nil
}

func (r *pgSyncCycleRepo) Complete(ctx context.Context, id string, headBoundary map[string]any, completionReason string) error {
	headPayload, err := json.Marshal(nonNilMap(headBoundary))
	if err != nil {
		return fmt.Errorf("Complete marshal head_boundary: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'complete',
		    last_hydrated_at = CASE
		        WHEN kind = 'hydration' THEN now()
		        ELSE last_hydrated_at
		    END,
		    head_boundary = CASE
		        WHEN head_boundary = '{}'::jsonb AND $2::jsonb <> '{}'::jsonb THEN $2::jsonb
		        ELSE head_boundary
		    END,
		    completion_reason = NULLIF($3, ''),
		    completed_at = now()
		WHERE id = $1::uuid
	`, id, headPayload, completionReason)
	if err != nil {
		return fmt.Errorf("Complete: %w", err)
	}
	return nil
}

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
