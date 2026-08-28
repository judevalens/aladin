package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
	"time"

	"aladin/backend_v2/internal/dbtest"
	"github.com/pressly/goose/v3"
)

func TestSessionBoundContentTokenMigration(t *testing.T) {
	base := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("content_token_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	}()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	conn, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, conn, "migrations", 49, goose.WithAllowMissing()); err != nil {
		t.Fatal(err)
	}
	exec := func(query string) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	// Existing login + shard + old unbound credential, before migration 50.
	exec(`INSERT INTO users(id,email,created_at,updated_at) VALUES ('11111111-1111-1111-1111-111111111111','token-test@example.com',now(),now())`)
	exec(`INSERT INTO user_sessions(user_id,token_hash,expires_at) VALUES ('11111111-1111-1111-1111-111111111111','session',now()+interval '30 days')`)
	exec(`INSERT INTO artifacts(id,user_id,type,title,content) VALUES ('migration-shard','11111111-1111-1111-1111-111111111111','app','Shard','')`)
	exec(`INSERT INTO content_tokens(token_hash,user_id,expires_at) VALUES ('legacy','11111111-1111-1111-1111-111111111111',now()+interval '12 hours')`)
	if err := goose.UpToContext(ctx, conn, "migrations", 50, goose.WithAllowMissing()); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT count(*) FROM content_tokens`, 0},
		{`SELECT count(*) FROM user_sessions WHERE token_hash='session' AND revoked_at IS NULL`, 1},
		{`SELECT count(*) FROM artifacts WHERE id='migration-shard'`, 1},
	} {
		var count int
		if err := conn.QueryRowContext(ctx, check.query).Scan(&count); err != nil || count != check.want {
			t.Fatalf("%s: got %d want %d, err=%v", check.query, count, check.want, err)
		}
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO content_tokens(token_hash,user_id,expires_at) VALUES ('unbound','11111111-1111-1111-1111-111111111111',now()+interval '30 days')`); err == nil {
		t.Fatal("migration allows a credential with no issuing session")
	}
	exec(`INSERT INTO content_tokens(token_hash,user_id,expires_at,session_token_hash) SELECT 'bound',user_id,expires_at,token_hash FROM user_sessions WHERE token_hash='session'`)
	if err := goose.DownToContext(ctx, conn, "migrations", 49); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM content_tokens`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback left long-lived credentials for session-unaware code: n=%d err=%v", count, err)
	}
}
