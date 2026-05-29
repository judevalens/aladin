package repo

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testAdminUserID = "00000000-0000-0000-0000-000000000001"

func strptr(s string) *string { return &s }

// Exercises the change-feed core: append field-level rows under the per-user
// lock, then pull a coalesced delta. Skips if no Postgres is reachable.
func TestSyncChangeFeed_AppendAndCoalescedPull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aladin:password@localhost:5433/aladin?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("postgres not reachable: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewSyncPostgres(pool)
	const entityID = "test-sync-node-A"

	// Clean slate for this entity, and restore it after.
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM workspace_changes WHERE user_id = $1 AND entity_id = $2`,
			testAdminUserID, entityID)
	}
	cleanup()
	defer cleanup()

	// Cursor before our writes: pull-since-this returns only our appends.
	var cursor0 int64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM workspace_changes WHERE user_id = $1`,
		testAdminUserID).Scan(&cursor0); err != nil {
		t.Fatalf("cursor0: %v", err)
	}

	// One write txn: lock the user, append three field-level changes.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := LockUser(ctx, tx, testAdminUserID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	mut := "client-1:1"
	appends := []Change{
		{EntityKind: "node", EntityID: entityID, Op: OpUpdate, Field: strptr("title"), Value: json.RawMessage(`"v1"`), MutationID: &mut},
		{EntityKind: "node", EntityID: entityID, Op: OpUpdate, Field: strptr("title"), Value: json.RawMessage(`"v2"`), MutationID: &mut},
		{EntityKind: "node", EntityID: entityID, Op: OpUpdate, Field: strptr("body"), Value: json.RawMessage(`"x"`), MutationID: &mut},
	}
	var lastSeq int64
	for _, c := range appends {
		seq, err := AppendChange(ctx, tx, testAdminUserID, c)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("append: %v", err)
		}
		lastSeq = seq
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Pull the delta: title coalesced to v2 + body=x → 2 changes, seq-ordered.
	res, err := repo.PullDelta(ctx, testAdminUserID, cursor0)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("coalesce failed: got %d changes, want 2 (title latest + body): %+v", len(res.Changes), res.Changes)
	}
	// seq order
	if res.Changes[0].Seq >= res.Changes[1].Seq {
		t.Fatalf("not seq-ordered: %+v", res.Changes)
	}
	// title resolves to the latest value (v2), not v1
	var sawTitleV2, sawBody bool
	for _, c := range res.Changes {
		if c.Field != nil && *c.Field == "title" {
			if strings.TrimSpace(string(c.Value)) != `"v2"` {
				t.Fatalf("title coalesced to %s, want \"v2\"", string(c.Value))
			}
			sawTitleV2 = true
		}
		if c.Field != nil && *c.Field == "body" {
			sawBody = true
		}
	}
	if !sawTitleV2 || !sawBody {
		t.Fatalf("missing expected fields: title-v2=%v body=%v", sawTitleV2, sawBody)
	}
	// cursor advanced to the feed high-water
	if res.Cursor != lastSeq {
		t.Fatalf("cursor = %d, want high-water %d", res.Cursor, lastSeq)
	}

	// A pull from the new cursor returns nothing and holds the cursor.
	res2, err := repo.PullDelta(ctx, testAdminUserID, res.Cursor)
	if err != nil {
		t.Fatalf("pull2: %v", err)
	}
	if len(res2.Changes) != 0 || res2.Cursor != res.Cursor {
		t.Fatalf("idempotent re-pull failed: %+v", res2)
	}
}
