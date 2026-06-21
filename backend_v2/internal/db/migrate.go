package db

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate runs all pending goose migrations against the given pool.
// Called once at startup before the server begins accepting work.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

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
