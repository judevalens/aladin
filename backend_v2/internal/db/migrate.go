package db

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate runs all pending goose migrations against the given pool.
// Called once at startup before the server begins accepting work.
// migrationLockKey is the fixed advisory-lock key that serializes concurrent migrators.
const migrationLockKey int64 = 492700117

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	// Serialize concurrent migrators. api/worker/mcp (and ops backfills) each run Migrate on boot;
	// in dev they start independently, so two processes can race goose on the same pending migration
	// → duplicate DDL / a half-applied schema. A session-level advisory lock makes the second
	// migrator wait for the first. The lock is held on a dedicated connection and released when it
	// closes, so a crashed migrator can't deadlock the others (Postgres frees it on disconnect).
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: lock conn: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("migrate: acquire lock: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
			slog.Warn("migrate: advisory unlock failed", "err", err)
		}
	}()

	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}
	// AllowMissing applies any out-of-order pending migration (a lower version than the
	// DB's max) rather than erroring. The consolidated migration numbers are gappy
	// (00006, 00010-00012) and partially-migrated environments (e.g. a dev DB that got
	// 00010/00011 before 00006) must still pick up the missing ones on boot.
	if err := goose.UpContext(ctx, db, "migrations", goose.WithAllowMissing()); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}
