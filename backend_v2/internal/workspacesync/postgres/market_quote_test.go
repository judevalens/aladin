package postgres_test

import (
	"aladin/backend_v2/internal/workspacesync"
	"context"
	"encoding/json"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/realtime"
	workspacepostgres "aladin/backend_v2/internal/workspacesync/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAppendMarketQuoteDrainsAsBroadcast(t *testing.T) {
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
	sync := workspacepostgres.NewSyncPostgres(pool)

	// Cursor before the write, so DrainSince returns just our event window.
	before, err := sync.Horizon(ctx)
	if err != nil {
		t.Fatalf("horizon: %v", err)
	}
	q := market.Quote{Symbol: "NVDA", InstrumentID: "inst-nvda", Last: 1183.56}
	payload, _ := json.Marshal(q)
	if err := sync.AppendMarketQuote(ctx, q.InstrumentID, payload); err != nil {
		t.Fatalf("append: %v", err)
	}

	events, _, err := sync.DrainSince(ctx, before)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	var found *workspacesync.OutboxAppEvent
	for i := range events {
		if e := events[i].AppEvent; e != nil && e.Stream == realtime.MarketStream && e.ResourceID == "inst-nvda" {
			found = e
		}
	}
	if found == nil {
		t.Fatalf("market quote app_event not drained (got %d events)", len(events))
	}
	if found.ResourceKind != "quote" {
		t.Fatalf("resourceKind = %q, want quote", found.ResourceKind)
	}
}
