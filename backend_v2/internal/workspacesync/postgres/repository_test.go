package postgres

import (
	"aladin/backend_v2/internal/artifact"
	"context"
	"strconv"
	"testing"
	"time"

	artifactpostgres "aladin/backend_v2/internal/artifact/postgres"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/outbox"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/treesync"
	"aladin/backend_v2/internal/workspacesync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Data-layer R1 — server-side outbox tests. These are DB integration tests:
// they skip when no Postgres is reachable. They exercise the generic outbox
// (append + xid-horizon pull), the cold-start snapshot (incl. tombstones), and
// the producer write path (frame emit + soft delete + tombstone-hiding reads).

// testAdminUserID is UNIQUE PER TEST PROCESS. `go test ./...` runs packages as parallel processes
// against the ONE shared sandbox DB, so a hardcoded id (this was 00000000-…-000000000001, the same
// literal used by internal/api and internal/mcp) meant three packages read/wrote/deleted each
// other's rows mid-assertion — the source of the outbox flakes. A per-process id isolates them, and
// also ignores rows left by earlier runs.
var testAdminUserID = uuid.NewString()

// tid namespaces a tree_node/artifact id to this test process. tree_nodes.id and artifacts.id are
// GLOBAL primary keys (not scoped by user_id), so a fixed literal like "folder-a" collides with the
// same literal in another package's parallel run — and with rows left by earlier runs, now that
// cleanup is (correctly) user-scoped rather than a global wipe.
func tid(name string) string { return name + "-" + testRunSuffix }

var testRunSuffix = uuid.NewString()[:8]

func strptr(s string) *string { return &s }

// --- shared test infrastructure (used across repo integration tests) ---

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	// SAFETY: this test wipes workspace tables — only ever run it against an
	// explicit throwaway TEST_DATABASE_URL, never the dev DB.
	dsn := dbtest.RequireTestDSN(t)
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

// cleanupSyncTables gives THIS test's user a deterministic start. Scoped to testAdminUserID on
// purpose: an unscoped `DELETE FROM outbox_events` wipes rows other packages' tests are asserting
// on (they share the sandbox DB and run as parallel processes), which is exactly what made the
// outbox/drain tests flaky. Never widen this back to a global delete.
func cleanupSyncTables(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Separate Execs: a parameterised statement can't carry multiple commands.
	for _, q := range []string{
		`DELETE FROM outbox_events WHERE user_id = $1::uuid`,
		`DELETE FROM tree_nodes WHERE user_id = $1::uuid`,
		`DELETE FROM artifacts WHERE user_id = $1::uuid`,
	} {
		if _, err := pool.Exec(ctx, q, testAdminUserID); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
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

	ar := artifactpostgres.NewArtifactsPostgres(pool)
	title := "Folder A"
	folderA := tid("folder-a")
	if err := ar.CreateTreeNode(ctx, artifact.TreeNodeRecord{
		ID: folderA, Kind: "folder", Title: &title, Position: 1,
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
	var got workspacesync.FrameEntity
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
	if got.EntityID != folderA || got.Op != workspacesync.OpUpsert {
		t.Fatalf("entity = %+v, want upsert %s", got, folderA)
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

	ar := artifactpostgres.NewArtifactsPostgres(pool)
	sr := NewSyncPostgres(pool)

	t1 := "F1"
	if err := ar.CreateTreeNode(ctx, artifact.TreeNodeRecord{ID: tid("f1"), Kind: "folder", Title: &t1, Position: 1}); err != nil {
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
	if err := ar.CreateTreeNode(ctx, artifact.TreeNodeRecord{ID: tid("f2"), Kind: "folder", Title: &t2, Position: 2}); err != nil {
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

	ar := artifactpostgres.NewArtifactsPostgres(pool)
	keepTitle, dropTitle := "keep", "drop"
	keepID, dropID := tid("keep"), tid("drop")
	if err := ar.CreateTreeNode(ctx, artifact.TreeNodeRecord{ID: keepID, Kind: "folder", Title: &keepTitle, Position: 1}); err != nil {
		t.Fatalf("create keep: %v", err)
	}
	if err := ar.CreateTreeNode(ctx, artifact.TreeNodeRecord{ID: dropID, Kind: "folder", Title: &dropTitle, Position: 2}); err != nil {
		t.Fatalf("create drop: %v", err)
	}
	if err := ar.DeleteBrowserNode(ctx, dropID); err != nil {
		t.Fatalf("delete drop: %v", err)
	}

	// Normal reads hide the tombstone.
	folders, err := ar.ListAllFolders(ctx)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	for _, f := range folders {
		if f.ID == dropID {
			t.Fatalf("ListAllFolders returned tombstoned folder %q", dropID)
		}
	}

	// The snapshot includes BOTH, with 'drop' as a delete tombstone.
	src := treesync.NewTreeSyncSource(pool)
	ents, err := src.Snapshot(ctx, testAdminUserID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	byID := map[string]workspacesync.FrameEntity{}
	for _, e := range ents {
		byID[e.EntityID] = e
	}
	if e, ok := byID[keepID]; !ok || e.Op != workspacesync.OpUpsert {
		t.Fatalf("keep = %+v (ok=%v), want upsert", e, ok)
	}
	if e, ok := byID[dropID]; !ok || e.Op != workspacesync.OpDelete {
		t.Fatalf("drop = %+v (ok=%v), want delete tombstone", e, ok)
	}
	if byID[dropID].Seq <= byID[keepID].Seq {
		// drop was created then deleted → its seq must be strictly higher than
		// keep's single create, proving the delete bumped the version.
		t.Fatalf("drop seq %d should exceed keep seq %d (delete bumps seq)", byID[dropID].Seq, byID[keepID].Seq)
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
	if err := outbox.LockUser(ctxTO, tx, testAdminUserID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := appendOutboxEvent(ctxTO, tx, testAdminUserID, workspacesync.Frame{
		Entities: []workspacesync.FrameEntity{{EntityKind: "folder", EntityID: "inflight", Seq: 1, Op: workspacesync.OpUpsert}},
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

func countEntities(frames []workspacesync.Frame) int {
	n := 0
	for _, f := range frames {
		n += len(f.Entities)
	}
	return n
}

// TestOutbox_CursorNeverAdvancesPastInFlight is the adversarial gap test: a pull
// running CONCURRENTLY with an uncommitted write must not advance its returned
// cursor past the in-flight xid, and a later pull from that advanced cursor must
// still deliver the event once it commits. This is the property the "gap-free"
// claim rests on (see data-layer-offline-readable.md): horizon = pg_snapshot_xmin
// is provably <= any still-in-flight xid, because such an xid is "still active" in
// the snapshot. If the horizon were the max assigned xid (e.g. pg_current_xact_id
// or a BIGSERIAL) this test would catch the resulting permanent silent loss.
//
// pg_current_xact_id() assigns the xid at INSERT-evaluation time (not commit), so
// the in-flight xid genuinely exists below where a naive horizon could land —
// this test forces exactly that window.
func TestOutbox_CursorNeverAdvancesPastInFlight(t *testing.T) {
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

	// Open an uncommitted write tx and append a frame; capture its (now assigned,
	// still in-flight) xid.
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
	if err := outbox.LockUser(ctxTO, tx, testAdminUserID); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := appendOutboxEvent(ctxTO, tx, testAdminUserID, workspacesync.Frame{
		Entities: []workspacesync.FrameEntity{{EntityKind: "folder", EntityID: "gap-probe", Seq: 1, Op: workspacesync.OpUpsert}},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	var inFlightStr string
	if err := tx.QueryRow(ctxTO, `SELECT pg_current_xact_id()::text`).Scan(&inFlightStr); err != nil {
		t.Fatalf("read in-flight xid: %v", err)
	}
	inFlightXid, err := strconv.ParseUint(inFlightStr, 10, 64)
	if err != nil {
		t.Fatalf("parse in-flight xid %q: %v", inFlightStr, err)
	}

	// Pull WHILE the write is in flight. It must see nothing AND must not advance
	// its cursor at or past the in-flight xid (else the post-commit re-pull skips it).
	frames, cursor1, err := sr.PullSince(ctx, testAdminUserID, baseCursor)
	if err != nil {
		t.Fatalf("concurrent pull: %v", err)
	}
	if total := countEntities(frames); total != 0 {
		t.Fatalf("concurrent pull saw %d in-flight entities, want 0", total)
	}
	if cursor1 > inFlightXid {
		t.Fatalf("cursor advanced to %d, PAST the in-flight xid %d — silent gap", cursor1, inFlightXid)
	}

	// Commit, then re-pull FROM THE ADVANCED CURSOR (cursor1, not baseCursor). The
	// event must still arrive — proving the concurrent pull's cursor advance was safe.
	if err := tx.Commit(ctxTO); err != nil {
		t.Fatalf("commit: %v", err)
	}
	frames, _, err = sr.PullSince(ctx, testAdminUserID, cursor1)
	if err != nil {
		t.Fatalf("re-pull from advanced cursor: %v", err)
	}
	if total := countEntities(frames); total != 1 {
		t.Fatalf("re-pull from advanced cursor %d saw %d entities, want 1 — the in-flight event was silently lost", cursor1, total)
	}
}
