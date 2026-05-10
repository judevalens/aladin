package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgProviderStreamRepo struct{ pool *pgxpool.Pool }

func NewProviderStreamRepository(pool *pgxpool.Pool) ProviderStreamRepository {
	return &pgProviderStreamRepo{pool}
}

func (r *pgProviderStreamRepo) GetByID(ctx context.Context, id string) (*ProviderStream, error) {
	s := &ProviderStream{}
	var configJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, provider, stream_kind, stream_key, name, config, sync_status,
		       COALESCE((config->>'sync_interval_seconds')::int, 10) AS sync_interval,
		       last_picked_at, last_refresh_at
		  FROM provider_streams
		 WHERE id = $1::uuid
	`, id).Scan(&s.ID, &s.Provider, &s.StreamKind, &s.StreamKey, &s.Name, &configJSON, &s.SyncStatus, &s.SyncInterval, &s.LastPickedAt, &s.LastRefreshAt)
	if err != nil {
		return nil, fmt.Errorf("ProviderStream GetByID: %w", err)
	}
	_ = json.Unmarshal(configJSON, &s.Config)
	return s, nil
}

func (r *pgProviderStreamRepo) Ensure(ctx context.Context, stream *ProviderStream) (*ProviderStream, error) {
	config, err := json.Marshal(stream.Config)
	if err != nil {
		return nil, fmt.Errorf("ProviderStream Ensure marshal config: %w", err)
	}
	out := &ProviderStream{}
	var configJSON []byte
	err = r.pool.QueryRow(ctx, `
		INSERT INTO provider_streams (provider, stream_kind, stream_key, name, config)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (provider, stream_kind, stream_key) DO UPDATE
		SET name = EXCLUDED.name,
		    config = provider_streams.config || EXCLUDED.config,
		    updated_at = now()
		RETURNING id::text, provider, stream_kind, stream_key, name, config, sync_status,
		          COALESCE((config->>'sync_interval_seconds')::int, 10) AS sync_interval,
		          last_picked_at, last_refresh_at
	`, stream.Provider, stream.StreamKind, stream.StreamKey, stream.Name, string(config)).
		Scan(&out.ID, &out.Provider, &out.StreamKind, &out.StreamKey, &out.Name, &configJSON, &out.SyncStatus, &out.SyncInterval, &out.LastPickedAt, &out.LastRefreshAt)
	if err != nil {
		return nil, fmt.Errorf("ProviderStream Ensure: %w", err)
	}
	_ = json.Unmarshal(configJSON, &out.Config)
	return out, nil
}

func (r *pgProviderStreamRepo) ClaimBatch(ctx context.Context, limit int) ([]*ProviderStream, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE provider_streams
		SET sync_status = 'queued',
		    last_picked_at = now()
		WHERE id IN (
			SELECT id FROM provider_streams
			WHERE sync_mode = 'poll'
			  AND sync_state = 'active'
			  AND sync_status = 'idle'
			  AND (
			      EXISTS (
			          SELECT 1
			            FROM sync_cycles
			           WHERE provider_stream_id = provider_streams.id
			             AND status = 'active'
			             AND (
			                 kind <> 'hydration'
			                 OR last_hydrated_at IS NULL
			                 OR last_hydrated_at + (
			                     CASE
			                         WHEN (cursor->>'hydration_interval_seconds') ~ '^[0-9]+$'
			                         THEN (cursor->>'hydration_interval_seconds')::int
			                         ELSE 0
			                     END
			                     * interval '1 second'
			                 ) <= now()
			             )
			      )
			      OR last_refresh_at IS NULL
			      OR last_refresh_at + (
			         COALESCE((config->>'sync_interval_seconds')::int, 10) * interval '1 second'
			      ) <= now()
			  )
			ORDER BY COALESCE(last_picked_at, to_timestamp(0)) ASC,
			         COALESCE(last_refresh_at, to_timestamp(0)) ASC,
			         created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id::text, provider, stream_kind, stream_key, name, config, sync_status,
		          COALESCE((config->>'sync_interval_seconds')::int, 10) AS sync_interval,
		          last_picked_at, last_refresh_at
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("ProviderStream ClaimBatch: %w", err)
	}
	defer rows.Close()

	var streams []*ProviderStream
	for rows.Next() {
		s := &ProviderStream{}
		var configJSON []byte
		if err := rows.Scan(&s.ID, &s.Provider, &s.StreamKind, &s.StreamKey, &s.Name, &configJSON, &s.SyncStatus, &s.SyncInterval, &s.LastPickedAt, &s.LastRefreshAt); err != nil {
			return nil, fmt.Errorf("ProviderStream ClaimBatch scan: %w", err)
		}
		_ = json.Unmarshal(configJSON, &s.Config)
		streams = append(streams, s)
	}
	return streams, rows.Err()
}

func (r *pgProviderStreamRepo) MarkSyncStarted(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_streams
		SET sync_status = 'syncing',
		    last_sync_started_at = now()
		WHERE id = $1::uuid
	`, id)
	if err != nil {
		return fmt.Errorf("ProviderStream MarkSyncStarted: %w", err)
	}
	return nil
}

func (r *pgProviderStreamRepo) MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	return r.patchAndSetStatus(ctx, id, "idle", false, configUpdates)
}

func (r *pgProviderStreamRepo) MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	return r.patchAndSetStatus(ctx, id, "idle", true, configUpdates)
}

func (r *pgProviderStreamRepo) MarkSyncFailed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE provider_streams SET sync_status = 'idle' WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("ProviderStream MarkSyncFailed: %w", err)
	}
	return nil
}

func (r *pgProviderStreamRepo) Release(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE provider_streams SET sync_status = 'idle' WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("ProviderStream Release: %w", err)
	}
	return nil
}

func (r *pgProviderStreamRepo) patchAndSetStatus(ctx context.Context, id string, status string, markRefresh bool, configUpdates map[string]any) error {
	if configUpdates == nil {
		configUpdates = map[string]any{}
	}
	patch, err := json.Marshal(configUpdates)
	if err != nil {
		return fmt.Errorf("ProviderStream patch marshal: %w", err)
	}
	lastRefreshExpr := "last_refresh_at"
	if markRefresh {
		lastRefreshExpr = "now()"
	}
	_, err = r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE provider_streams
		SET sync_status = $2,
		    last_refresh_at = %s,
		    config = COALESCE(config, '{}'::jsonb) || $3::jsonb,
		    updated_at = now()
		WHERE id = $1::uuid
	`, lastRefreshExpr), id, status, string(patch))
	if err != nil {
		return fmt.Errorf("ProviderStream patch: %w", err)
	}
	return nil
}
