package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens the shared Postgres pool with explicit sizing, lifecycle, and server-side guards.
// The bare pgxpool.New defaults leave the pool unbounded-in-effect (a hung query holds a connection
// forever, with no ceiling control or health check). These settings bound that; all are
// env-overridable so a slow migration/backfill environment can loosen them.
//
// NOTE: this pool also runs goose migrations (via stdlib.OpenDBFromPool), so statement_timeout is
// generous by default (a single migration statement can be slow) and can be set to "0" to disable.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db parse config: %w", err)
	}

	// Pool sizing + lifecycle: a ceiling so a leak/hang can't exhaust connections; a periodic
	// health check + max lifetime recycle stale/half-dead conns behind a LB or after a failover.
	cfg.MaxConns = int32(envInt("DB_MAX_CONNS", 20))
	cfg.MinConns = int32(envInt("DB_MIN_CONNS", 2))
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	// Server-side guards (per connection): a lock wait or a leaked open transaction can't block
	// forever, and a runaway query is eventually aborted rather than pinning its connection.
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	setParam(cfg.ConnConfig.RuntimeParams, "statement_timeout", envStr("DB_STATEMENT_TIMEOUT", "120000")) // ms; 0 disables
	setParam(cfg.ConnConfig.RuntimeParams, "lock_timeout", envStr("DB_LOCK_TIMEOUT", "10000"))            // ms
	setParam(cfg.ConnConfig.RuntimeParams, "idle_in_transaction_session_timeout", envStr("DB_IDLE_TX_TIMEOUT", "60000"))

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

// setParam sets a Postgres runtime param only when it isn't already present in the DSN (so an
// explicit DSN option always wins) and the value is non-empty.
func setParam(params map[string]string, key, value string) {
	if value == "" {
		return
	}
	if _, ok := params[key]; ok {
		return
	}
	params[key] = value
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
