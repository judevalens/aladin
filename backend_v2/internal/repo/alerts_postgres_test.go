package repo_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAlertsAndNotificationsRoundTrip exercises migration 00030 + both repos, and the
// transactional Fire path (alert disarm + notification insert + outbox app_event in one tx).
func TestAlertsAndNotificationsRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("no TEST_DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID := uuid.NewString()
	instrumentID := uuid.NewString()
	symbol := "TST" + uuid.NewString()[:6] // unique per run (instruments has a unique active-symbol constraint)
	// Seed the user (outbox_events FKs to users) + the instrument (alerts FKs to instruments).
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $1 || '@test.local', now())
	`, userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO instruments (instrument_id, symbol, name) VALUES ($1::uuid, $2, 'Test Co')
	`, instrumentID, symbol); err != nil {
		t.Fatalf("seed instrument: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM alerts WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM instruments WHERE instrument_id = $1::uuid`, instrumentID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	alerts := repo.NewAlertsPostgres(pool)
	notes := repo.NewNotificationsPostgres(pool)

	alertID := uuid.NewString()
	a := coreservice.Alert{
		ID: alertID, UserID: userID, InstrumentID: instrumentID, Symbol: symbol,
		Direction: "above", Threshold: 200, Armed: true, Status: "active",
	}
	if err := alerts.Insert(ctx, a); err != nil {
		t.Fatalf("insert alert: %v", err)
	}

	// ListByUser + ListActive both surface it.
	byUser, err := alerts.ListByUser(ctx, userID)
	if err != nil || len(byUser) != 1 || byUser[0].Symbol != symbol || !byUser[0].Armed {
		t.Fatalf("ListByUser = %+v err=%v", byUser, err)
	}
	active, err := alerts.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if !containsAlert(active, alertID) {
		t.Fatalf("ListActive missing the alert")
	}

	// Fire: transactional disarm + notification + outbox event.
	beforeOutbox := countOutbox(t, ctx, pool, userID)
	data, _ := json.Marshal(map[string]any{"alertId": alertID, "symbol": "TSTALERT", "price": 200.14})
	n := coreservice.Notification{
		UserID: userID, Kind: "price_alert", Title: "TSTALERT crossed above 200.00",
		Body: "Last 200.14", Data: data,
	}
	if err := alerts.Fire(ctx, alertID, 200.14, time.Now(), n); err != nil {
		t.Fatalf("fire: %v", err)
	}

	// Alert disarmed + trigger recorded.
	byUser, _ = alerts.ListByUser(ctx, userID)
	if byUser[0].Armed {
		t.Fatal("alert should be disarmed after fire")
	}
	if byUser[0].LastFiredPrice != 200.14 {
		t.Fatalf("last_fired_price = %v, want 200.14", byUser[0].LastFiredPrice)
	}
	// Notification row written + surfaced as unread.
	unread, err := notes.ListUnread(ctx, userID)
	if err != nil || len(unread) != 1 || unread[0].Kind != "price_alert" {
		t.Fatalf("ListUnread = %+v err=%v", unread, err)
	}
	// The fire tx also appended exactly one outbox app_event (the live transport).
	if got := countOutbox(t, ctx, pool, userID) - beforeOutbox; got != 1 {
		t.Fatalf("fire appended %d outbox events, want 1", got)
	}

	// Firing again is idempotent — already disarmed, no duplicate notification.
	if err := alerts.Fire(ctx, alertID, 201, time.Now(), n); err != nil {
		t.Fatalf("second fire: %v", err)
	}
	if unread, _ := notes.ListUnread(ctx, userID); len(unread) != 1 {
		t.Fatalf("second fire on a disarmed alert should not add a notification, got %d", len(unread))
	}

	// MarkRead clears unread.
	if err := notes.MarkRead(ctx, userID, unread[0].ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if u, _ := notes.ListUnread(ctx, userID); len(u) != 0 {
		t.Fatalf("expected 0 unread after MarkRead, got %d", len(u))
	}

	// Delete removes the alert.
	if err := alerts.Delete(ctx, userID, alertID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if byUser, _ := alerts.ListByUser(ctx, userID); len(byUser) != 0 {
		t.Fatalf("alert not deleted, got %d", len(byUser))
	}
}

func containsAlert(as []coreservice.Alert, id string) bool {
	for _, a := range as {
		if a.ID == id {
			return true
		}
	}
	return false
}

func countOutbox(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE user_id = $1::uuid`, userID).Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return n
}
