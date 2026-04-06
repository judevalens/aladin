package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSyncCycleRepo struct{ pool *pgxpool.Pool }

func NewSyncCycleRepository(pool *pgxpool.Pool) SyncCycleRepository {
	return &pgSyncCycleRepo{pool: pool}
}

func (r *pgSyncCycleRepo) ListActiveBySource(ctx context.Context, sourceID string) ([]*SyncCycle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, source_id::text, kind, status, cursor, covered_until,
		       last_picked_at, created_at, completed_at
		  FROM sync_cycles
		 WHERE source_id = $1::uuid
		   AND status IN ('active', 'running')
		 ORDER BY created_at ASC
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("ListActiveBySource: %w", err)
	}
	defer rows.Close()

	var cycles []*SyncCycle
	for rows.Next() {
		cycle := &SyncCycle{}
		var cursorJSON []byte
		var coveredJSON []byte
		if err := rows.Scan(
			&cycle.ID,
			&cycle.SourceID,
			&cycle.Kind,
			&cycle.Status,
			&cursorJSON,
			&coveredJSON,
			&cycle.LastPickedAt,
			&cycle.CreatedAt,
			&cycle.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("ListActiveBySource scan: %w", err)
		}
		_ = json.Unmarshal(cursorJSON, &cycle.Cursor)
		_ = json.Unmarshal(coveredJSON, &cycle.CoveredUntil)
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
	covered, err := json.Marshal(nonNilMap(cycle.CoveredUntil))
	if err != nil {
		return fmt.Errorf("Create marshal covered_until: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO sync_cycles (id, source_id, kind, status, cursor, covered_until)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb, $6::jsonb)
	`, cycle.ID, cycle.SourceID, cycle.Kind, cycle.Status, cursor, covered)
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

func (r *pgSyncCycleRepo) UpdateCursor(ctx context.Context, id string, cursor map[string]any) error {
	payload, err := json.Marshal(nonNilMap(cursor))
	if err != nil {
		return fmt.Errorf("UpdateCursor marshal: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'active',
		    cursor = $2::jsonb
		WHERE id = $1::uuid
	`, id, payload)
	if err != nil {
		return fmt.Errorf("UpdateCursor: %w", err)
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

func (r *pgSyncCycleRepo) Complete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sync_cycles
		SET status = 'complete',
		    completed_at = now()
		WHERE id = $1::uuid
	`, id)
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
