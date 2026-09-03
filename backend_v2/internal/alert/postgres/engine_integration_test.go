package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/alert"
	alertpostgres "aladin/backend_v2/internal/alert/postgres"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/market"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type engineFakeDemand struct{}

func (engineFakeDemand) Subscribe(context.Context, []string) error   { return nil }
func (engineFakeDemand) Unsubscribe(context.Context, []string) error { return nil }

type engineFakeSnap map[string]float64

func (f engineFakeSnap) FetchSnapshot(_ context.Context, sym string) (market.Quote, bool, error) {
	if p, ok := f[sym]; ok {
		return market.Quote{Symbol: sym, Last: p}, true, nil
	}
	return market.Quote{}, false, nil
}

// TestAlertEngineIntegration wires the REAL AlertEngine to the REAL Postgres repos and drives it
// with injected ticks — proving the whole fire path end to end: reconcile picks up a DB alert,
// a live crossing series fires it via the transactional Fire, and a durable notification +
// outbox app_event land. No Alpaca/WS needed (OnTick is called directly).
func TestAlertEngineIntegration(t *testing.T) {
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
	symbol := "TST" + uuid.NewString()[:6]
	mustExec(t, ctx, pool, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $1||'@t.local', now())`, userID)
	mustExec(t, ctx, pool, `INSERT INTO instruments (instrument_id, symbol, name) VALUES ($1::uuid, $2, 'T')`, instrumentID, symbol)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE user_id=$1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM alerts WHERE user_id=$1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE user_id=$1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM instruments WHERE instrument_id=$1::uuid`, instrumentID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1::uuid`, userID)
	})

	alertRepo := alertpostgres.NewAlertsPostgres(pool)
	// Armed alert (below threshold at the seed price) — will fire on the live up-cross.
	if err := alertRepo.Insert(ctx, alert.Alert{
		ID: uuid.NewString(), UserID: userID, InstrumentID: instrumentID, Symbol: symbol,
		Direction: alert.AlertAbove, Threshold: 200, Armed: true, Status: "active",
	}); err != nil {
		t.Fatalf("insert alert: %v", err)
	}

	eng := alert.NewAlertEngine(alertRepo, engineFakeDemand{}, engineFakeSnap{symbol: 195})
	ectx, cancel := context.WithCancel(ctx)
	defer cancel()
	eng.Start(ectx)
	time.Sleep(150 * time.Millisecond) // let the first reconcile seed slope state from the 195 snapshot

	// Feed a genuine up-cross through the observer (as the WS goroutine would).
	for _, p := range []float64{196, 197, 198, 199, 200.5, 201} {
		eng.OnTick(symbol, p, time.Now())
		time.Sleep(3 * time.Millisecond)
	}

	// The fire lands a durable notification + an outbox app_event.
	notes := alertpostgres.NewNotificationsPostgres(pool)
	deadline := time.Now().Add(3 * time.Second)
	var unread []alert.Notification
	for time.Now().Before(deadline) {
		unread, _ = notes.ListUnread(ctx, userID)
		if len(unread) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(unread) != 1 || unread[0].Kind != "price_alert" {
		t.Fatalf("expected 1 price_alert notification, got %+v", unread)
	}
	// Alert disarmed in the DB, and the outbox carries the live transport event.
	var armed bool
	if err := pool.QueryRow(ctx, `SELECT armed FROM alerts WHERE user_id=$1::uuid`, userID).Scan(&armed); err != nil {
		t.Fatalf("read alert: %v", err)
	}
	if armed {
		t.Fatal("alert should be disarmed after firing")
	}
	if countOutbox(t, ctx, pool, userID) != 1 {
		t.Fatalf("expected 1 outbox event from the fire, got %d", countOutbox(t, ctx, pool, userID))
	}
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}
