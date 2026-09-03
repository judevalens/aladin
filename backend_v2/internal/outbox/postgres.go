// Package outbox owns the PostgreSQL transaction primitives that preserve
// canonical-data and event atomicity across domain persistence adapters.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
)

// LockUser serializes a user's writes so outbox transaction IDs become visible
// in commit order. The advisory lock is released automatically at transaction end.
func LockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, userID); err != nil {
		return fmt.Errorf("sync: lock user: %w", err)
	}
	return nil
}

// AppendData appends one durable data frame inside the caller's canonical
// mutation transaction. An empty frame is a no-op.
func AppendData(ctx context.Context, tx pgx.Tx, userID string, frame service.Frame) error {
	if len(frame.Entities) == 0 {
		return nil
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("sync: marshal frame: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (user_id, type, payload)
		VALUES ($1::uuid, 'data_event', $2::jsonb)
	`, userID, payload); err != nil {
		return fmt.Errorf("sync: append outbox event: %w", err)
	}
	return nil
}

// AppendApp appends one live application event inside the caller's mutation
// transaction. Application events are not returned by durable data pulls.
func AppendApp(ctx context.Context, tx pgx.Tx, userID string, event service.OutboxAppEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("sync: marshal app event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (user_id, type, payload)
		VALUES ($1::uuid, 'app_event', $2::jsonb)
	`, userID, payload); err != nil {
		return fmt.Errorf("sync: append app event: %w", err)
	}
	return nil
}
