package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	workspacepostgres "aladin/backend_v2/internal/workspacesync/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPullSince_SkipsPoisonRow proves one undecodable outbox payload no longer wedges a user's
// pull: PullSince skips it (advancing past it) and still returns the surrounding valid frames.
// Before the fix, a single bad row failed the whole batch and halted delivery for that user.
func TestPullSince_SkipsPoisonRow(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $1 || '@test.local', now())`, userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox_events WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	validFrame := `{"entities":[{"entityKind":"folder","entityId":"f1","seq":"1","op":"upsert"}]}`
	// A JSON string is valid jsonb but fails json.Unmarshal into workspacesync.Frame (a struct) — the
	// realistic "poison" shape (a producer wrote a payload the reader can't decode).
	poison := `"poison"`
	for _, p := range []string{validFrame, poison, validFrame} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO outbox_events (user_id, type, payload) VALUES ($1::uuid, 'data_event', $2::jsonb)`,
			userID, p); err != nil {
			t.Fatalf("insert outbox row: %v", err)
		}
	}

	sync := workspacepostgres.NewSyncPostgres(pool)
	frames, cursor, err := sync.PullSince(ctx, userID, 0)
	if err != nil {
		t.Fatalf("PullSince returned an error instead of skipping the poison row: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2 (the poison row skipped, both valid frames kept)", len(frames))
	}
	if cursor == 0 {
		t.Fatalf("cursor did not advance past the window")
	}
}

// TestPruneOutbox_DeletesOldKeepsRecent proves retention deletes rows past the window and keeps
// recent ones, so the durable log stays bounded.
func TestPruneOutbox_DeletesOldKeepsRecent(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $1 || '@test.local', now())`, userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox_events WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events (user_id, type, payload, created_at)
		VALUES ($1::uuid, 'data_event', '{"entities":[]}'::jsonb, now() - interval '10 minutes')`, userID); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events (user_id, type, payload)
		VALUES ($1::uuid, 'data_event', '{"entities":[]}'::jsonb)`, userID); err != nil {
		t.Fatalf("insert recent: %v", err)
	}

	if _, err := workspacepostgres.NewSyncPostgres(pool).PruneOutbox(ctx, 5*time.Minute, 5000); err != nil {
		t.Fatalf("prune: %v", err)
	}

	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE user_id=$1::uuid`, userID).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("after prune this user has %d rows, want 1 (old pruned, recent kept)", remaining)
	}
}
