package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"aladin/backend_v2/internal/providerconnection"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresProviderConnectionRepository struct{ pool *pgxpool.Pool }

func NewProviderConnectionPostgres(pool *pgxpool.Pool) *PostgresProviderConnectionRepository {
	return &PostgresProviderConnectionRepository{pool: pool}
}

func (r *PostgresProviderConnectionRepository) ListProviderConnections(ctx context.Context, userID string) ([]providerconnection.ProviderConnection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, user_id::text, provider, backend, provider_config_key,
		       external_connection_id, status, granted_scopes, metadata,
		       created_at, updated_at, disconnected_at
		  FROM provider_connections
		 WHERE user_id = $1::uuid
		   AND status = 'active'
		 ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []providerconnection.ProviderConnection
	for rows.Next() {
		rec, err := scanProviderConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresProviderConnectionRepository) UpsertProviderConnection(ctx context.Context, userID string, input providerconnection.ProviderConnection) (providerconnection.ProviderConnection, error) {
	metadata, _ := json.Marshal(input.Metadata)
	grantedScopes := input.GrantedScopes
	if grantedScopes == nil {
		grantedScopes = []string{}
	}
	row := r.pool.QueryRow(ctx, `
		WITH existing AS (
		    SELECT id
		      FROM provider_connections
		     WHERE user_id = $1::uuid
		       AND backend = $2
		       AND provider_config_key = $3
		       AND external_connection_id = $4
		     ORDER BY updated_at DESC
		     LIMIT 1
		), updated AS (
		    UPDATE provider_connections pc
		       SET provider = $5,
		           status = 'active',
		           granted_scopes = $6,
		           metadata = $7::jsonb,
		           disconnected_at = NULL,
		           updated_at = now()
		      FROM existing
		     WHERE pc.id = existing.id
		    RETURNING pc.id::text, pc.user_id::text, pc.provider, pc.backend, pc.provider_config_key,
		              pc.external_connection_id, pc.status, pc.granted_scopes, pc.metadata,
		              pc.created_at, pc.updated_at, pc.disconnected_at
		), inserted AS (
		    INSERT INTO provider_connections (
		        user_id, provider, backend, provider_config_key, external_connection_id,
		        status, granted_scopes, metadata
		    )
		    SELECT $1::uuid, $5, $2, $3, $4, 'active', $6, $7::jsonb
		     WHERE NOT EXISTS (SELECT 1 FROM updated)
		    RETURNING id::text, user_id::text, provider, backend, provider_config_key,
		              external_connection_id, status, granted_scopes, metadata,
		              created_at, updated_at, disconnected_at
		)
		SELECT * FROM updated
		UNION ALL
		SELECT * FROM inserted
		LIMIT 1
	`, userID, input.Backend, input.ProviderConfigKey, input.ExternalConnectionID,
		input.Provider, grantedScopes, string(metadata))
	return scanProviderConnection(row)
}

func (r *PostgresProviderConnectionRepository) DisconnectProviderConnection(ctx context.Context, userID string, id string) (providerconnection.ProviderConnection, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE provider_connections
		   SET status = 'disconnected',
		       disconnected_at = COALESCE(disconnected_at, now()),
		       updated_at = now()
		 WHERE id = $1::uuid
		   AND user_id = $2::uuid
		   AND status = 'active'
		RETURNING id::text, user_id::text, provider, backend, provider_config_key,
		          external_connection_id, status, granted_scopes, metadata,
		          created_at, updated_at, disconnected_at
	`, id, userID)
	rec, err := scanProviderConnection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return providerconnection.ProviderConnection{}, coreservice.ErrNotFound
		}
		return providerconnection.ProviderConnection{}, err
	}
	return rec, nil
}

func (r *PostgresProviderConnectionRepository) DisableProviderStreamsForConnection(ctx context.Context, userID string, connectionID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_streams
		   SET sync_state = 'inactive',
		       sync_status = 'idle',
		       updated_at = now()
		 WHERE owner_user_id = $1::uuid
		   AND provider_connection_id = $2::uuid
	`, userID, connectionID)
	return err
}

type providerConnectionScanner interface {
	Scan(dest ...any) error
}

func scanProviderConnection(row providerConnectionScanner) (providerconnection.ProviderConnection, error) {
	var rec providerconnection.ProviderConnection
	var metadata []byte
	var createdAt time.Time
	var updatedAt time.Time
	var disconnectedAt *time.Time
	if err := row.Scan(
		&rec.ID,
		&rec.UserID,
		&rec.Provider,
		&rec.Backend,
		&rec.ProviderConfigKey,
		&rec.ExternalConnectionID,
		&rec.Status,
		&rec.GrantedScopes,
		&metadata,
		&createdAt,
		&updatedAt,
		&disconnectedAt,
	); err != nil {
		return providerconnection.ProviderConnection{}, err
	}
	_ = json.Unmarshal(metadata, &rec.Metadata)
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	rec.UpdatedAt = updatedAt.Format(time.RFC3339)
	if disconnectedAt != nil {
		v := disconnectedAt.Format(time.RFC3339)
		rec.DisconnectedAt = &v
	}
	return rec, nil
}
