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
