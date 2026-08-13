package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAuthRepository struct{ pool *pgxpool.Pool }

func NewAuthPostgres(pool *pgxpool.Pool) *PostgresAuthRepository {
	return &PostgresAuthRepository{pool: pool}
}

func (r *PostgresAuthRepository) CreateUser(ctx context.Context, email string, passwordHash string) (coreservice.CurrentUser, error) {
	var user coreservice.CurrentUser
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, now(), now())
		RETURNING id::text, email
	`, uuid.NewString(), email, passwordHash).Scan(&user.ID, &user.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return coreservice.CurrentUser{}, coreservice.BadRequest("email is already registered")
		}
		return coreservice.CurrentUser{}, fmt.Errorf("Auth CreateUser: %w", err)
	}
	return user, nil
}

func (r *PostgresAuthRepository) GetUserByEmail(ctx context.Context, email string) (coreservice.AuthUserRecord, error) {
	var user coreservice.AuthUserRecord
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, email, COALESCE(password_hash, '')
		  FROM users
		 WHERE lower(email) = lower($1)
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.AuthUserRecord{}, coreservice.ErrNotFound
		}
		return coreservice.AuthUserRecord{}, fmt.Errorf("Auth GetUserByEmail: %w", err)
	}
	return user, nil
}

func (r *PostgresAuthRepository) CreateSession(ctx context.Context, rec coreservice.AuthSessionRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_sessions (user_id, token_hash, expires_at, user_agent)
		VALUES ($1::uuid, $2, $3, $4)
	`, rec.UserID, rec.TokenHash, rec.ExpiresAt, rec.UserAgent)
	if err != nil {
		return fmt.Errorf("Auth CreateSession: %w", err)
	}
	return nil
}

func (r *PostgresAuthRepository) GetUserBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (coreservice.CurrentUser, error) {
	var user coreservice.CurrentUser
	err := r.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email
		  FROM user_sessions s
		  JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1
		   AND s.revoked_at IS NULL
		   AND s.expires_at > $2
	`, tokenHash, now).Scan(&user.ID, &user.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.CurrentUser{}, coreservice.ErrUnauthenticated
		}
		return coreservice.CurrentUser{}, fmt.Errorf("Auth GetUserBySessionTokenHash: %w", err)
	}
	return user, nil
}

func (r *PostgresAuthRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_sessions
		   SET revoked_at = COALESCE(revoked_at, now())
		 WHERE token_hash = $1
	`, tokenHash)
	if err != nil {
		return fmt.Errorf("Auth RevokeSession: %w", err)
	}
	return nil
}

func (r *PostgresAuthRepository) TouchSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_sessions
		   SET last_seen_at = now()
		 WHERE token_hash = $1
		   AND revoked_at IS NULL
	`, tokenHash)
	if err != nil {
		return fmt.Errorf("Auth TouchSession: %w", err)
	}
	return nil
}

func (r *PostgresAuthRepository) MarkLogin(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users
		   SET last_login_at = now(),
		       updated_at = now()
		 WHERE id = $1::uuid
	`, userID)
	if err != nil {
		return fmt.Errorf("Auth MarkLogin: %w", err)
	}
	return nil
}

func (r *PostgresAuthRepository) CreateIntegrationToken(ctx context.Context, rec coreservice.IntegrationTokenRecord) (coreservice.IntegrationToken, error) {
	var token coreservice.IntegrationToken
	err := r.pool.QueryRow(ctx, `
		INSERT INTO integration_tokens (user_id, name, token_hash, scopes, status, expires_at)
		VALUES ($1::uuid, $2, $3, $4, 'active', $5)
		RETURNING id::text, name, scopes, status, expires_at, created_at, last_used_at
	`, rec.UserID, rec.Name, rec.TokenHash, rec.Scopes, rec.ExpiresAt).Scan(
		&token.ID,
		&token.Name,
		&token.Scopes,
		&token.Status,
		&token.ExpiresAt,
		&token.CreatedAt,
		&token.LastUsedAt,
	)
	if err != nil {
		return coreservice.IntegrationToken{}, fmt.Errorf("Auth CreateIntegrationToken: %w", err)
	}
	return token, nil
}

func (r *PostgresAuthRepository) ListIntegrationTokens(ctx context.Context, userID string) ([]coreservice.IntegrationToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, scopes, status, expires_at, created_at, last_used_at
		  FROM integration_tokens
		 WHERE user_id = $1::uuid
		 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("Auth ListIntegrationTokens: %w", err)
	}
	defer rows.Close()

	out := []coreservice.IntegrationToken{}
	for rows.Next() {
		var token coreservice.IntegrationToken
		if err := rows.Scan(
			&token.ID,
			&token.Name,
			&token.Scopes,
			&token.Status,
			&token.ExpiresAt,
			&token.CreatedAt,
			&token.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("Auth ListIntegrationTokens scan: %w", err)
		}
		out = append(out, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Auth ListIntegrationTokens rows: %w", err)
	}
	return out, nil
}

func (r *PostgresAuthRepository) RevokeIntegrationToken(ctx context.Context, userID string, id string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE integration_tokens
		   SET status = 'revoked',
		       revoked_at = COALESCE(revoked_at, now()),
		       updated_at = now()
		 WHERE id = $1::uuid
		   AND user_id = $2::uuid
		   AND status = 'active'
	`, id, userID)
	if err != nil {
		return fmt.Errorf("Auth RevokeIntegrationToken: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return coreservice.ErrNotFound
	}
	return nil
}

func (r *PostgresAuthRepository) GetIntegrationTokenByHash(ctx context.Context, tokenHash string, now time.Time) (coreservice.IntegrationTokenPrincipalRecord, error) {
	var rec coreservice.IntegrationTokenPrincipalRecord
	err := r.pool.QueryRow(ctx, `
		SELECT t.id::text, u.id::text, u.email, t.scopes
		  FROM integration_tokens t
		  JOIN users u ON u.id = t.user_id
		 WHERE t.token_hash = $1
		   AND t.status = 'active'
		   AND t.revoked_at IS NULL
		   AND (t.expires_at IS NULL OR t.expires_at > $2)
	`, tokenHash, now).Scan(&rec.ID, &rec.UserID, &rec.Email, &rec.Scopes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.IntegrationTokenPrincipalRecord{}, coreservice.ErrUnauthenticated
		}
		return coreservice.IntegrationTokenPrincipalRecord{}, fmt.Errorf("Auth GetIntegrationTokenByHash: %w", err)
	}
	return rec, nil
}

func (r *PostgresAuthRepository) TouchIntegrationToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE integration_tokens
		   SET last_used_at = now(),
		       updated_at = now()
		 WHERE token_hash = $1
		   AND status = 'active'
		   AND revoked_at IS NULL
	`, tokenHash)
	if err != nil {
		return fmt.Errorf("Auth TouchIntegrationToken: %w", err)
	}
	return nil
}

func (r *PostgresAuthRepository) CreateContentToken(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO content_tokens (token_hash, user_id, expires_at)
		VALUES ($1, $2::uuid, $3)
	`, tokenHash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("Auth CreateContentToken: %w", err)
	}
	return nil
}

func (r *PostgresAuthRepository) GetContentTokenUser(ctx context.Context, tokenHash string, now time.Time) (coreservice.CurrentUser, error) {
	var user coreservice.CurrentUser
	err := r.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email
		  FROM content_tokens t
		  JOIN users u ON u.id = t.user_id
		 WHERE t.token_hash = $1
		   AND t.expires_at > $2
	`, tokenHash, now).Scan(&user.ID, &user.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.CurrentUser{}, coreservice.ErrUnauthenticated
		}
		return coreservice.CurrentUser{}, fmt.Errorf("Auth GetContentTokenUser: %w", err)
	}
	return user, nil
}

func (r *PostgresAuthRepository) DeleteExpiredContentTokens(ctx context.Context, now time.Time) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM content_tokens WHERE expires_at <= $1`, now)
	if err != nil {
		return fmt.Errorf("Auth DeleteExpiredContentTokens: %w", err)
	}
	return nil
}
