// Package dbtest is a SAFETY guard for Postgres-backed tests.
//
// Several integration tests TRUNCATE / DELETE workspace tables to get a
// deterministic start. If they default to DATABASE_URL (the dev DB), a plain
// `go test ./...` silently wipes the developer's real data — which has bitten
// us. RequireTestDSN forces those tests to run ONLY against an explicit,
// throwaway TEST_DATABASE_URL that is distinct from DATABASE_URL; otherwise the
// test skips and touches nothing.
package dbtest

import (
	"os"
	"strings"
	"testing"
)

// RequireTestDSN returns the destructive-test database DSN, or skips the test.
//
//   - TEST_DATABASE_URL unset  -> t.Skip (no DB is opened, nothing is wiped)
//   - TEST_DATABASE_URL == DATABASE_URL -> t.Fatal (refuse to wipe the dev DB)
//
// Point TEST_DATABASE_URL at a disposable database (e.g. an ephemeral docker
// Postgres) to run these tests.
func RequireTestDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("DB test skipped: set TEST_DATABASE_URL to a THROWAWAY database to run it " +
			"(these tests wipe tables and must never touch DATABASE_URL / the dev DB)")
	}
	if dev := strings.TrimSpace(os.Getenv("DATABASE_URL")); dev != "" && dsn == dev {
		t.Fatal("TEST_DATABASE_URL must differ from DATABASE_URL — these tests wipe data")
	}
	return dsn
}
