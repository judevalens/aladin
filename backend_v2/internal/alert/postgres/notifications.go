package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/alert"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresNotificationRepository is the durable per-user inbox. Create is transactional +
// self-transporting: it inserts the row AND appends the outbox app_event (workspace stream,
// resource kind "notification") in one tx, so the notification is durable (this table, fetched
// on load) and delivered live (the OutboxDrainer → realtime) atomically — no realtime coupling
// in the producer. Mirrors the shard-build transactional-outbox shape.
type PostgresNotificationRepository struct{ pool *pgxpool.Pool }

func NewNotificationsPostgres(pool *pgxpool.Pool) *PostgresNotificationRepository {
	return &PostgresNotificationRepository{pool: pool}
}

func (r *PostgresNotificationRepository) Create(ctx context.Context, n alert.Notification) (alert.Notification, error) {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	data := n.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return alert.Notification{}, fmt.Errorf("notification begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var createdAt string
	if err := tx.QueryRow(ctx, `
		INSERT INTO notifications (id, user_id, kind, title, body, data)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb)
		RETURNING to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
	`, n.ID, n.UserID, n.Kind, n.Title, n.Body, string(data)).Scan(&createdAt); err != nil {
		return alert.Notification{}, fmt.Errorf("notification insert: %w", err)
	}

	// Live transport: a tenant-scoped workspace app_event the drainer routes to this user.
	payload, err := json.Marshal(alert.NotificationCreatedPayload{
		ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body, Data: data, CreatedAt: createdAt,
	})
	if err != nil {
		return alert.Notification{}, fmt.Errorf("notification marshal payload: %w", err)
	}
	if err := appendNotificationEvent(ctx, tx, n.UserID, n.ID, payload); err != nil {
		return alert.Notification{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return alert.Notification{}, fmt.Errorf("notification commit: %w", err)
	}

	n.Data = data
	n.CreatedAt = createdAt
	return n, nil
}

func (r *PostgresNotificationRepository) List(ctx context.Context, userID string, limit int) ([]alert.Notification, error) {
	return r.query(ctx, `
		SELECT id::text, kind, title, body, data,
		       COALESCE(to_char(read_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS read_at,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		  FROM notifications
		 WHERE user_id = $1::uuid
		 ORDER BY created_at DESC
		 LIMIT $2
	`, userID, limit)
}

func (r *PostgresNotificationRepository) ListUnread(ctx context.Context, userID string) ([]alert.Notification, error) {
	return r.query(ctx, `
		SELECT id::text, kind, title, body, data, '' AS read_at,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		  FROM notifications
		 WHERE user_id = $1::uuid AND read_at IS NULL
		 ORDER BY created_at DESC
	`, userID)
}

func (r *PostgresNotificationRepository) query(ctx context.Context, sql string, args ...any) ([]alert.Notification, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("notification list: %w", err)
	}
	defer rows.Close()
	out := make([]alert.Notification, 0)
	for rows.Next() {
		var n alert.Notification
		var data []byte
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &data, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("notification scan: %w", err)
		}
		if len(data) > 0 {
			n.Data = json.RawMessage(data)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *PostgresNotificationRepository) MarkRead(ctx context.Context, userID, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now()
		 WHERE id = $1::uuid AND user_id = $2::uuid AND read_at IS NULL
	`, id, userID)
	if err != nil {
		return fmt.Errorf("notification mark read: %w", err)
	}
	return nil
}
