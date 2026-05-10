package repo

import (
	"context"
	"encoding/json"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultKGName = "Default Research Graph"

type PostgresSourceRepository struct{ pool *pgxpool.Pool }

func NewSourcePostgres(pool *pgxpool.Pool) *PostgresSourceRepository {
	return &PostgresSourceRepository{pool: pool}
}

func (r *PostgresSourceRepository) EnsureUserAndGraph(ctx context.Context, userID string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE users
		   SET updated_at = now()
		 WHERE id = $1::uuid
	`, userID)
	if err != nil {
		return "", err
	}

	var kgID string
	err = tx.QueryRow(ctx, `
		WITH existing AS (
		    SELECT id::text
		      FROM knowledge_graphs
		     WHERE user_id = $1::uuid
		     ORDER BY created_at ASC
		     LIMIT 1
		), inserted AS (
		    INSERT INTO knowledge_graphs (user_id, name, description)
		    SELECT $1::uuid, $2, 'Default workspace for persisted feed sources.'
		    WHERE NOT EXISTS (SELECT 1 FROM existing)
		    RETURNING id::text
		)
		SELECT id FROM existing
		UNION ALL
		SELECT id FROM inserted
		LIMIT 1
	`, userID, defaultKGName).Scan(&kgID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return kgID, nil
}

func (r *PostgresSourceRepository) List(ctx context.Context, userID string) ([]coreservice.SourceRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ss.id::text, ss.name, ps.provider, ps.sync_mode, ss.status, ps.config,
		       0.85::float, 0.60::float, ss.created_at, ps.last_refresh_at
		  FROM source_subscriptions ss
		  JOIN provider_streams ps ON ps.id = ss.provider_stream_id
		 WHERE ss.user_id = $1::uuid
		 ORDER BY ss.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]coreservice.SourceRecord, 0)
	for rows.Next() {
		var rec coreservice.SourceRecord
		var configJSON []byte
		var createdAt time.Time
		var lastSynced *time.Time
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.Type, &rec.SyncMode, &rec.SyncState, &configJSON,
			&rec.AutoPromoteThreshold, &rec.SuggestThreshold, &createdAt, &lastSynced,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(configJSON, &rec.Config)
		rec.CreatedAt = createdAt.Format(time.RFC3339)
		if lastSynced != nil {
			v := lastSynced.Format(time.RFC3339)
			rec.LastSyncedAt = &v
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresSourceRepository) Create(ctx context.Context, sourceID string, userID string, kgID string, payload *coreservice.SourcePayload) (coreservice.SourceRecord, error) {
	configJSON, _ := json.Marshal(payload.Config)
	policyJSON, _ := json.Marshal(payload.Policy)
	if payload.StreamKind == "" {
		payload.StreamKind = "default"
	}
	if payload.StreamKey == "" {
		payload.StreamKey = payload.Name
	}
	_, err := r.pool.Exec(ctx, `
		WITH stream AS (
		    INSERT INTO provider_streams (
		        provider, stream_kind, stream_key, name, sync_mode, sync_state, config
		    )
		    VALUES ($1, $2, $3, $4, $5, 'active', $6::jsonb)
		    ON CONFLICT (provider, stream_kind, stream_key) WHERE owner_user_id IS NULL DO UPDATE
		    SET name = EXCLUDED.name,
		        config = provider_streams.config || EXCLUDED.config,
		        updated_at = now()
		    RETURNING id
		)
		INSERT INTO source_subscriptions (
		    id, user_id, kg_id, provider_stream_id, name, policy, status
		)
		SELECT $7::uuid, $8::uuid, $9::uuid, stream.id, $4, $10::jsonb, 'active'
		  FROM stream
		ON CONFLICT (user_id, kg_id, provider_stream_id) DO UPDATE
		SET name = EXCLUDED.name,
		    policy = source_subscriptions.policy || EXCLUDED.policy,
		    status = 'active',
		    updated_at = now()
	`, payload.Type, payload.StreamKind, payload.StreamKey, payload.Name, payload.SyncMode, string(configJSON),
		sourceID, userID, kgID, string(policyJSON))
	if err != nil {
		return coreservice.SourceRecord{}, err
	}

	row := r.pool.QueryRow(ctx, `
		SELECT ss.id::text, ss.name, ps.provider, ps.sync_mode, ss.status, ps.config,
		       0.85::float, 0.60::float, ss.created_at, ps.last_refresh_at
		  FROM source_subscriptions ss
		  JOIN provider_streams ps ON ps.id = ss.provider_stream_id
		 WHERE ss.id = $1::uuid
	`, sourceID)
	var rec coreservice.SourceRecord
	var configOut []byte
	var createdAt time.Time
	var lastSynced *time.Time
	if err := row.Scan(
		&rec.ID, &rec.Name, &rec.Type, &rec.SyncMode, &rec.SyncState, &configOut,
		&rec.AutoPromoteThreshold, &rec.SuggestThreshold, &createdAt, &lastSynced,
	); err != nil {
		return coreservice.SourceRecord{}, err
	}
	_ = json.Unmarshal(configOut, &rec.Config)
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	if lastSynced != nil {
		v := lastSynced.Format(time.RFC3339)
		rec.LastSyncedAt = &v
	}
	return rec, nil
}

func (r *PostgresSourceRepository) Delete(ctx context.Context, id string, userID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM source_subscriptions
		 WHERE id = $1::uuid AND user_id = $2::uuid
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return coreservice.ErrNotFound
	}
	return nil
}
