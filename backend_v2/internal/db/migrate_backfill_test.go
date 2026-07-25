package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestWatchlistBackfillMigration exercises the one path the sandbox's already-migrated DB can't:
// applying 00031 on top of EXISTING flat watchlist_items rows and confirming they land in a
// per-user default list. Runs the whole chain up to 00030, seeds old-shape rows, then applies
// 00031 on a throwaway database so nothing shared is touched.
func TestWatchlistBackfillMigration(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	// Create a throwaway database on the same server (CREATE DATABASE can't run in a tx).
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	tmpName := fmt.Sprintf("wl_backfill_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+tmpName); err != nil {
		admin.Close()
		t.Fatalf("create temp db: %v", err)
	}
	t.Cleanup(func() {
		// Terminate any lingering backends, then drop.
		_, _ = admin.ExecContext(context.Background(),
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1", tmpName)
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+tmpName)
		admin.Close()
	})

	tmpDSN := swapDBName(base, tmpName)
	conn, err := sql.Open("pgx", tmpDSN)
	if err != nil {
		t.Fatalf("open temp: %v", err)
	}
	defer conn.Close()

	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	// Migrate to the version just before named watchlists — watchlist_items is still the flat
	// (user_id, instrument_id) table here.
	if err := goose.UpToContext(ctx, conn, "migrations", 30, goose.WithAllowMissing()); err != nil {
		t.Fatalf("up to 30: %v", err)
	}

	// Seed two users, each with an instrument on their flat watchlist (one user has two items).
	userA := "11111111-1111-1111-1111-111111111111"
	userB := "22222222-2222-2222-2222-222222222222"
	inst1 := mustInstrument(t, ctx, conn, "BKFL1")
	inst2 := mustInstrument(t, ctx, conn, "BKFL2")
	inst3 := mustInstrument(t, ctx, conn, "BKFL3")
	seedItem(t, ctx, conn, userA, inst1)
	seedItem(t, ctx, conn, userA, inst2)
	seedItem(t, ctx, conn, userB, inst3)

	// Apply 00031 — the backfill under test.
	if err := goose.UpContext(ctx, conn, "migrations", goose.WithAllowMissing()); err != nil {
		t.Fatalf("up (apply 00031): %v", err)
	}

	// Each distinct user got exactly one default manual list.
	for _, u := range []string{userA, userB} {
		var n int
		var name, kind string
		if err := conn.QueryRowContext(ctx,
			"SELECT count(*) FROM watchlists WHERE user_id=$1", u).Scan(&n); err != nil {
			t.Fatalf("count lists: %v", err)
		}
		if n != 1 {
			t.Fatalf("user %s: want 1 default list, got %d", u, n)
		}
		if err := conn.QueryRowContext(ctx,
			"SELECT name, kind FROM watchlists WHERE user_id=$1", u).Scan(&name, &kind); err != nil {
			t.Fatalf("read list: %v", err)
		}
		if name != "Watchlist" || kind != "manual" {
			t.Fatalf("user %s: default list = (%q,%q), want (Watchlist,manual)", u, name, kind)
		}
	}

	// Every pre-existing item now points at its owner's default list (no NULLs, right owner).
	var orphans int
	if err := conn.QueryRowContext(ctx, `
		SELECT count(*) FROM watchlist_items wi
		 LEFT JOIN watchlists wl ON wl.id = wi.watchlist_id
		 WHERE wi.watchlist_id IS NULL OR wl.user_id <> wi.user_id`).Scan(&orphans); err != nil {
		t.Fatalf("orphan check: %v", err)
	}
	if orphans != 0 {
		t.Fatalf("want 0 mis-attached items after backfill, got %d", orphans)
	}

	// userA's default list holds both of their instruments.
	var aCount int
	if err := conn.QueryRowContext(ctx, `
		SELECT count(*) FROM watchlist_items wi
		 JOIN watchlists wl ON wl.id = wi.watchlist_id
		 WHERE wl.user_id=$1`, userA).Scan(&aCount); err != nil {
		t.Fatalf("count A items: %v", err)
	}
	if aCount != 2 {
		t.Fatalf("userA: want 2 items in default list, got %d", aCount)
	}
}

func mustInstrument(t *testing.T, ctx context.Context, conn *sql.DB, symbol string) string {
	t.Helper()
	var id string
	if err := conn.QueryRowContext(ctx,
		"INSERT INTO instruments (symbol) VALUES ($1) RETURNING instrument_id", symbol).Scan(&id); err != nil {
		t.Fatalf("seed instrument %s: %v", symbol, err)
	}
	return id
}

func seedItem(t *testing.T, ctx context.Context, conn *sql.DB, userID, instrumentID string) {
	t.Helper()
	if _, err := conn.ExecContext(ctx,
		"INSERT INTO watchlist_items (user_id, instrument_id) VALUES ($1, $2)", userID, instrumentID); err != nil {
		t.Fatalf("seed item: %v", err)
	}
}

// swapDBName replaces the database path of a postgres URL DSN with a new name.
func swapDBName(dsn, name string) string {
	i := strings.LastIndex(dsn, "/")
	if i < 0 {
		return dsn
	}
	rest := dsn[i+1:]
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		return dsn[:i+1] + name + rest[q:]
	}
	return dsn[:i+1] + name
}
