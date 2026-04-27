package repo

import (
	"context"
	"encoding/json"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultUserID = "00000000-0000-0000-0000-000000000001"
	defaultKGName = "Default Research Graph"
)

type PostgresSourceRepository struct{ pool *pgxpool.Pool }

func NewSourcePostgres(pool *pgxpool.Pool) *PostgresSourceRepository {
	return &PostgresSourceRepository{pool: pool}
}

func (r *PostgresSourceRepository) EnsureDefaultUserAndGraph(ctx context.Context) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, created_at)
		VALUES ($1::uuid, 'dev@aladin.local', now())
		ON CONFLICT (id) DO NOTHING
	`, defaultUserID)
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
	`, defaultUserID, defaultKGName).Scan(&kgID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return kgID, nil
}

func (r *PostgresSourceRepository) List(ctx context.Context) ([]coreservice.SourceRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, type, sync_mode, sync_state, config,
		       auto_promote_threshold, suggest_threshold, created_at,
		       last_synced_at
		  FROM sources
		 WHERE user_id = $1::uuid
		 ORDER BY created_at DESC
	`, defaultUserID)
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

func (r *PostgresSourceRepository) Create(ctx context.Context, sourceID string, kgID string, payload *coreservice.SourcePayload) (coreservice.SourceRecord, error) {
	configJSON, _ := json.Marshal(payload.Config)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sources (
		    id, user_id, kg_id, name, type, sync_mode, sync_state, config
		) VALUES (
		    $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, 'active', $7::jsonb
		)
	`, sourceID, defaultUserID, kgID, payload.Name, payload.Type, payload.SyncMode, string(configJSON))
	if err != nil {
		return coreservice.SourceRecord{}, err
	}

	row := r.pool.QueryRow(ctx, `
		SELECT id::text, name, type, sync_mode, sync_state, config,
		       auto_promote_threshold, suggest_threshold, created_at,
		       last_synced_at
		  FROM sources
		 WHERE id = $1::uuid
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

func (r *PostgresSourceRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM sources
		 WHERE id = $1::uuid AND user_id = $2::uuid
	`, id, defaultUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return coreservice.ErrNotFound
	}
	return nil
}
