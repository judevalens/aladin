package repo

import (
	"context"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Data-layer R1 — server-side outbox tests. These are DB integration tests:
// they skip when no Postgres is reachable. They exercise the generic outbox
// (append + xid-horizon pull), the cold-start snapshot (incl. tombstones), and
// the producer write path (frame emit + soft delete + tombstone-hiding reads).

const testAdminUserID = "00000000-0000-0000-0000-000000000001"

func strptr(s string) *string { return &s }

// --- shared test infrastructure (used across repo integration tests) ---

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = "postgres://aladin:password@localhost:5433/aladin"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func seedUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, created_at, updated_at)
		VALUES ($1::uuid, $2, now(), now())
		ON CONFLICT (id) DO NOTHING
	`, userID, userID+"@example.com")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

// cleanupSyncTables clears the sync-relevant tables for a deterministic start
// (the dev DB is shared). outbox_events + the canonical tables.
func cleanupSyncTables(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `DELETE FROM outbox_events; DELETE FROM tree_nodes; DELETE FROM artifacts`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func adminContext(userID string) context.Context {
	return coreservice.WithPrincipal(context.Background(), coreservice.Principal{
		UserID:    userID,
		ActorType: coreservice.ActorTypeUserSession,
		ActorID:   userID,
		Scopes:    []string{coreservice.ScopeArtifactsRead, coreservice.ScopeArtifactsWrite},
	})
}

// --- tests ---

// A folder create through the producer path emits exactly one frame, readable
// via the xid-horizon pull, carrying the entity's light upsert with seq >= 1.
func TestOutbox_ProducerEmitsFrameOnCreate(t *testing.T) {
	ctx := adminContext(testAdminUserID)
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool := mustTestPool(ctxTO, t)
	defer pool.Close()
	cleanupSyncTables(ctxTO, t, pool)
	seedUser(ctxTO, t, pool, testAdminUserID)

	ar := NewArtifactsPostgres(pool)
	title := "Folder A"
	if err := ar.CreateTreeNode(ctx, coreservice.TreeNodeRecord{
		ID: "folder-a", Kind: "folder", Title: &title, Position: 1,
	}); err != nil {
		t.Fatalf("create tree node: %v", err)
	}

	sr := NewSyncPostgres(pool)
	frames, horizon, err := sr.PullSince(ctx, testAdminUserID, 0)
	if err != nil {
		t.Fatalf("pull since: %v", err)
	}
	if horizon == 0 {
		t.Fatalf("horizon = 0, want > 0")
	}
	var got coreservice.FrameEntity
	n := 0
	for _, f := range frames {
		for _, e := range f.Entities {
			n++
			got = e
		}
	}
	if n != 1 {
		t.Fatalf("entity count = %d, want 1", n)
	}
	if got.EntityID != "folder-a" || got.Op != coreservice.OpUpsert {
		t.Fatalf("entity = %+v, want upsert folder-a", got)
	}
	if got.Seq == 0 {
		t.Fatalf("seq = 0, want >= 1 (bumped on write)")
	}
}

// The xid-horizon pull tiles perfectly: pulling from the previous horizon
// returns only events committed since, and the new horizon advances.
func TestOutbox_PullSinceFromCursorIsIncremental(t *testing.T) {
	ctx := adminContext(testAdminUserID)
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool := mustTestPool(ctxTO, t)
	defer pool.Close()
	cleanupSyncTables(ctxTO, t, pool)
	seedUser(ctxTO, t, pool, testAdminUserID)

	ar := NewArtifactsPostgres(pool)
	sr := NewSyncPostgres(pool)

	t1 := "F1"
	if err := ar.CreateTreeNode(ctx, coreservice.TreeNodeRecord{ID: "f1", Kind: "folder", Title: &t1, Position: 1}); err != nil {
		t.Fatalf("create f1: %v", err)
	}
	_, cursor, err := sr.PullSince(ctx, testAdminUserID, 0)
	if err != nil {
		t.Fatalf("pull 1: %v", err)
	}

	// Nothing new since the cursor.
	frames, cursor2, err := sr.PullSince(ctx, testAdminUserID, cursor)
	if err != nil {
		t.Fatalf("pull 2: %v", err)
	}
	if total := countEntities(frames); total != 0 {
		t.Fatalf("incremental pull with no new writes returned %d entities, want 0", total)
	}
	if cursor2 < cursor {
		t.Fatalf("horizon went backwards: %d < %d", cursor2, cursor)
	}

	// A second write shows up only in the next incremental pull.
	t2 := "F2"
	if err := ar.CreateTreeNode(ctx, coreservice.TreeNodeRecord{ID: "f2", Kind: "folder", Title: &t2, Position: 2}); err != nil {
		t.Fatalf("create f2: %v", err)
	}
	frames, _, err = sr.PullSince(ctx, testAdminUserID, cursor2)
	if err != nil {
		t.Fatalf("pull 3: %v", err)
	}
	if total := countEntities(frames); total != 1 {
		t.Fatalf("incremental pull after one write returned %d entities, want 1", total)
	}
}

// The cold-start snapshot includes tombstones (deleted entities as Op:delete),
// so a fresh client keeps resurrection blocked; live entities come back as
// upserts and are hidden from the normal reads.
func TestTreeSyncSource_SnapshotIncludesTombstones(t *testing.T) {
	ctx := adminContext(testAdminUserID)
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool := mustTestPool(ctxTO, t)
	defer pool.Close()
	cleanupSyncTables(ctxTO, t, pool)
	seedUser(ctxTO, t, pool, testAdminUserID)

	ar := NewArtifactsPostgres(pool)
	keepTitle, dropTitle := "keep", "drop"
	if err := ar.CreateTreeNode(ctx, coreservice.TreeNodeRecord{ID: "keep", Kind: "folder", Title: &keepTitle, Position: 1}); err != nil {
		t.Fatalf("create keep: %v", err)
	}
	if err := ar.CreateTreeNode(ctx, coreservice.TreeNodeRecord{ID: "drop", Kind: "folder", Title: &dropTitle, Position: 2}); err != nil {
		t.Fatalf("create drop: %v", err)
	}
	if err := ar.DeleteBrowserNode(ctx, "drop"); err != nil {
		t.Fatalf("delete drop: %v", err)
	}

	// Normal reads hide the tombstone.
	folders, err := ar.ListAllFolders(ctx)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	for _, f := range folders {
		if f.ID == "drop" {
			t.Fatalf("ListAllFolders returned tombstoned folder 'drop'")
		}
	}

	// The snapshot includes BOTH, with 'drop' as a delete tombstone.
	src := NewTreeSyncSource(pool)
	ents, err := src.Snapshot(ctx, testAdminUserID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	byID := map[string]coreservice.FrameEntity{}
	for _, e := range ents {
		byID[e.EntityID] = e
	}
	if e, ok := byID["keep"]; !ok || e.Op != coreservice.OpUpsert {
		t.Fatalf("keep = %+v (ok=%v), want upsert", e, ok)
	}
	if e, ok := byID["drop"]; !ok || e.Op != coreservice.OpDelete {
		t.Fatalf("drop = %+v (ok=%v), want delete tombstone", e, ok)
	}
	if byID["drop"].Seq <= byID["keep"].Seq {
		// drop was created then deleted → its seq must be strictly higher than
		// keep's single create, proving the delete bumped the version.
		t.Fatalf("drop seq %d should exceed keep seq %d (delete bumps seq)", byID["drop"].Seq, byID["keep"].Seq)
	}
}

// An in-flight (uncommitted) write is invisible to the horizon pull until it
// commits — the gap-free property (pg_snapshot_xmin excludes in-progress xids).
func TestOutbox_HorizonExcludesInFlight(t *testing.T) {
	ctx := adminContext(testAdminUserID)
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool := mustTestPool(ctxTO, t)
	defer pool.Close()
	cleanupSyncTables(ctxTO, t, pool)
	seedUser(ctxTO, t, pool, testAdminUserID)

	sr := NewSyncPostgres(pool)
	_, baseCursor, err := sr.PullSince(ctx, testAdminUserID, 0)
	if err != nil {
		t.Fatalf("base pull: %v", err)
	}

	// Open a write tx on a dedicated connection and append an outbox event, but
	// DON'T commit — its xid is now in-flight.
	conn, err := pool.Acquire(ctxTO)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	tx, err := conn.Begin(ctxTO)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctxTO) }()
	if err := LockUser(ctxTO, tx, testAdminUserID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := appendOutboxEvent(ctxTO, tx, testAdminUserID, coreservice.Frame{
		Entities: []coreservice.FrameEntity{{EntityKind: "folder", EntityID: "inflight", Seq: 1, Op: coreservice.OpUpsert}},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	// A concurrent pull must NOT see the in-flight row.
	frames, _, err := sr.PullSince(ctx, testAdminUserID, baseCursor)
	if err != nil {
		t.Fatalf("concurrent pull: %v", err)
	}
	if total := countEntities(frames); total != 0 {
		t.Fatalf("pull saw %d in-flight entities, want 0 (horizon must exclude uncommitted)", total)
	}

	// After commit, the next pull sees it.
	if err := tx.Commit(ctxTO); err != nil {
		t.Fatalf("commit: %v", err)
	}
	frames, _, err = sr.PullSince(ctx, testAdminUserID, baseCursor)
	if err != nil {
		t.Fatalf("post-commit pull: %v", err)
	}
	if total := countEntities(frames); total != 1 {
		t.Fatalf("post-commit pull saw %d entities, want 1", total)
	}
}

func countEntities(frames []coreservice.Frame) int {
	n := 0
	for _, f := range frames {
		n += len(f.Entities)
	}
	return n
}
