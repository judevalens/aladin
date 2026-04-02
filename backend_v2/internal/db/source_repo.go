package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgSourceRepo struct{ pool *pgxpool.Pool }

func NewSourceRepository(pool *pgxpool.Pool) SourceRepository {
	return &pgSourceRepo{pool}
}

func (r *pgSourceRepo) GetByID(ctx context.Context, id string) (*Source, error) {
	s := &Source{}
	var configJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, kg_id::text, name, type, config FROM sources WHERE id = $1::uuid`, id,
	).Scan(&s.ID, &s.KgID, &s.Name, &s.Type, &configJSON)
	if err != nil {
		return nil, fmt.Errorf("GetByID: %w", err)
	}
	_ = json.Unmarshal(configJSON, &s.Config)
	return s, nil
}

// ClaimBatch atomically marks up to limit due sources as 'queued' and returns them.
// Pure persistence — callers decide what jobs to build from the claimed sources.
// FOR UPDATE SKIP LOCKED ensures N concurrent schedulers never claim the same source.
func (r *pgSourceRepo) ClaimBatch(ctx context.Context, limit int) ([]*Source, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE sources
		SET sync_status = 'queued'
		WHERE id IN (
			SELECT id FROM sources
			WHERE sync_mode   = 'poll'
			  AND sync_state  = 'active'
			  AND sync_status = 'idle'
			  AND (
			      last_synced_at IS NULL
			      OR last_synced_at + (
			             COALESCE((config->>'sync_interval_seconds')::int, 3600)
			             * interval '1 second'
			         ) <= now()
			  )
			ORDER BY last_synced_at ASC NULLS FIRST
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id::text, kg_id::text, name, type, config, sync_status,
		          COALESCE((config->>'sync_interval_seconds')::int, 3600) AS sync_interval
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("ClaimBatch: %w", err)
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		s := &Source{}
		var configJSON []byte
		if err := rows.Scan(
			&s.ID, &s.KgID, &s.Name, &s.Type, &configJSON,
			&s.SyncStatus, &s.SyncInterval,
		); err != nil {
			return nil, fmt.Errorf("ClaimBatch scan: %w", err)
		}
		_ = json.Unmarshal(configJSON, &s.Config)
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// MarkSyncStarted transitions a source to 'syncing' and stamps last_sync_started_at.
func (r *pgSourceRepo) MarkSyncStarted(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sources
		SET sync_status          = 'syncing',
		    last_sync_started_at = now()
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("MarkSyncStarted: %w", err)
	}
	return nil
}

// MarkSynced resets a source to idle and stamps last_synced_at = now.
// Called by the sync handler after a full sync cycle completes.
func (r *pgSourceRepo) MarkSynced(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sources
		SET sync_status    = 'idle',
		    last_synced_at = now()
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("MarkSynced: %w", err)
	}
	return nil
}

// MarkSyncFailed resets a source to idle without updating last_synced_at.
// Makes it immediately eligible for retry on the next scheduler tick.
func (r *pgSourceRepo) MarkSyncFailed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sources
		SET sync_status = 'idle'
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("MarkSyncFailed: %w", err)
	}
	return nil
}
