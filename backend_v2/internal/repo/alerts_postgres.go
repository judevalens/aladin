package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAlertRepository persists price alerts. Fire is the transactional heart: it disarms
// the alert, inserts the notification, and appends the outbox app_event in ONE tx — all-or-
// nothing, so a mid-fire crash never leaves a disarmed-but-unnotified alert or a duplicate.
type PostgresAlertRepository struct{ pool *pgxpool.Pool }

func NewAlertsPostgres(pool *pgxpool.Pool) *PostgresAlertRepository {
	return &PostgresAlertRepository{pool: pool}
}

func (r *PostgresAlertRepository) Insert(ctx context.Context, a coreservice.Alert) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO alerts (id, user_id, instrument_id, symbol, direction, threshold, armed, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8)
	`, a.ID, a.UserID, a.InstrumentID, a.Symbol, a.Direction, a.Threshold, a.Armed, a.Status)
	if err != nil {
		return fmt.Errorf("alert insert: %w", err)
	}
	return nil
}

func (r *PostgresAlertRepository) ListByUser(ctx context.Context, userID string) ([]coreservice.Alert, error) {
	return r.query(ctx, `
		WHERE user_id = $1::uuid
		 ORDER BY created_at DESC
	`, userID)
}

// ListActive returns every active alert across all users — the engine's reconcile load.
func (r *PostgresAlertRepository) ListActive(ctx context.Context) ([]coreservice.Alert, error) {
	return r.query(ctx, `WHERE status = 'active'`)
}

func (r *PostgresAlertRepository) query(ctx context.Context, where string, args ...any) ([]coreservice.Alert, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, user_id::text, instrument_id::text, symbol, direction, threshold, armed, status,
		       COALESCE(to_char(last_fired_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS last_fired_at,
		       COALESCE(last_fired_price, 0),
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		  FROM alerts
	`+where, args...)
	if err != nil {
		return nil, fmt.Errorf("alert list: %w", err)
	}
	defer rows.Close()
	out := make([]coreservice.Alert, 0)
	for rows.Next() {
		var a coreservice.Alert
		if err := rows.Scan(&a.ID, &a.UserID, &a.InstrumentID, &a.Symbol, &a.Direction, &a.Threshold,
			&a.Armed, &a.Status, &a.LastFiredAt, &a.LastFiredPrice, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("alert scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Fire disarms the alert, records the trigger, inserts the notification, and appends the outbox
// event — atomically. If any step fails the whole fire rolls back (no partial state).
func (r *PostgresAlertRepository) Fire(ctx context.Context, alertID string, price float64, at time.Time, n coreservice.Notification) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	data := n.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("alert fire begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Disarm only if still armed — guards against a double-fire racing two ticks for the same
	// alert (the second UPDATE affects 0 rows and we bail without a duplicate notification).
	tag, err := tx.Exec(ctx, `
		UPDATE alerts SET armed = false, last_fired_at = $2, last_fired_price = $3
		 WHERE id = $1::uuid AND armed = true
	`, alertID, at, price)
	if err != nil {
		return fmt.Errorf("alert fire update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return tx.Rollback(ctx) // already fired/disarmed — not an error
	}

	var createdAt string
	if err := tx.QueryRow(ctx, `
		INSERT INTO notifications (id, user_id, kind, title, body, data)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb)
		RETURNING to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
	`, n.ID, n.UserID, n.Kind, n.Title, n.Body, string(data)).Scan(&createdAt); err != nil {
		return fmt.Errorf("alert fire notification: %w", err)
	}

	payload, err := json.Marshal(coreservice.NotificationCreatedPayload{
		ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body, Data: data, CreatedAt: createdAt,
	})
	if err != nil {
		return fmt.Errorf("alert fire marshal: %w", err)
	}
	if err := appendNotificationEvent(ctx, tx, n.UserID, n.ID, payload); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("alert fire commit: %w", err)
	}
	return nil
}

func (r *PostgresAlertRepository) Delete(ctx context.Context, userID, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM alerts WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID)
	if err != nil {
		return fmt.Errorf("alert delete: %w", err)
	}
	return nil
}

func (r *PostgresAlertRepository) SetStatus(ctx context.Context, userID, id, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE alerts SET status = $3 WHERE id = $1::uuid AND user_id = $2::uuid
	`, id, userID, status)
	if err != nil {
		return fmt.Errorf("alert set status: %w", err)
	}
	return nil
}
