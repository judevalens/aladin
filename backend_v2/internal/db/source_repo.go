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
		`SELECT id::text, kg_id::text, name, type, config, sync_status,
		        COALESCE((config->>'sync_interval_seconds')::int, 10) AS sync_interval,
		        last_picked_at, last_refresh_at
		   FROM sources
		  WHERE id = $1::uuid`, id,
	).Scan(&s.ID, &s.KgID, &s.Name, &s.Type, &configJSON, &s.SyncStatus, &s.SyncInterval, &s.LastPickedAt, &s.LastRefreshAt)
	if err != nil {
		return nil, fmt.Errorf("GetByID: %w", err)
	}
	_ = json.Unmarshal(configJSON, &s.Config)
	return s, nil
}

// ClaimBatch atomically marks up to limit due sources as 'queued' and returns them.
// Pure persistence — callers decide what jobs to build from the claimed sources.
// A source is eligible if either:
// - a new refresh is due, or
// - it already has an active (paused) cycle that needs another turn.
// FOR UPDATE SKIP LOCKED ensures N concurrent schedulers never claim the same source.
func (r *pgSourceRepo) ClaimBatch(ctx context.Context, limit int) ([]*Source, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE sources
		SET sync_status    = 'queued',
		    last_picked_at = now()
		WHERE id IN (
			SELECT id FROM sources
			WHERE sync_mode   = 'poll'
			  AND sync_state  = 'active'
			  AND sync_status = 'idle'
			  AND (
			      EXISTS (
			          SELECT 1
			            FROM sync_cycles
			           WHERE source_id = sources.id
			             AND status = 'active'
			      )
			      OR last_refresh_at IS NULL
			      OR last_refresh_at + (
			             COALESCE((config->>'sync_interval_seconds')::int, 10)
			             * interval '1 second'
			         ) <= now()
			  )
			ORDER BY COALESCE(last_picked_at, to_timestamp(0)) ASC,
			         COALESCE(last_refresh_at, to_timestamp(0)) ASC,
			         created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id::text, kg_id::text, name, type, config, sync_status,
		          COALESCE((config->>'sync_interval_seconds')::int, 10) AS sync_interval,
		          last_picked_at, last_refresh_at
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
			&s.SyncStatus, &s.SyncInterval, &s.LastPickedAt, &s.LastRefreshAt,
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

// MarkSyncPage resets a source to idle after one page without updating last_synced_at,
// making it immediately re-claimable for the next page.
func (r *pgSourceRepo) MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	if configUpdates == nil {
		configUpdates = map[string]any{}
	}
	patch, err := json.Marshal(configUpdates)
	if err != nil {
		return fmt.Errorf("MarkSyncPage marshal config updates: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE sources
		SET sync_status = 'idle',
		    config      = COALESCE(config, '{}'::jsonb) || $2::jsonb
		WHERE id = $1::uuid
	`, id, patch)
	if err != nil {
		return fmt.Errorf("MarkSyncPage: %w", err)
	}
	return nil
}

// MarkSynced resets a source to idle, stamps last_synced_at = now, and
// optionally merges top-level config updates into the source config.
func (r *pgSourceRepo) MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	if configUpdates == nil {
		configUpdates = map[string]any{}
	}
	patch, err := json.Marshal(configUpdates)
	if err != nil {
		return fmt.Errorf("MarkSynced marshal config updates: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE sources
		SET sync_status    = 'idle',
		    last_synced_at = now(),
		    last_refresh_at = now(),
		    config         = COALESCE(config, '{}'::jsonb) || $2::jsonb
		WHERE id = $1::uuid
	`, id, patch)
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

func (r *pgSourceRepo) Release(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sources
		SET sync_status = 'idle'
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("Release: %w", err)
	}
	return nil
}
