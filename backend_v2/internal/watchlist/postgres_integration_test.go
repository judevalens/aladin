package watchlist_test

import (
	"context"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/watchlist"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestWatchlistsRoundTrip exercises migration 00031 + the named-watchlist service/repo end to
// end: multiple lists, the SAME instrument in two lists (multi-list), item counts, rename,
// delete-cascades-items, ownership rejection, the default-list resolution, and the
// ResolveInstruments universe port (manual resolves; screener is reserved).
func TestWatchlistsRoundTrip(t *testing.T) {
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
	otherUser := uuid.NewString()
	instA := uuid.NewString()
	instB := uuid.NewString()
	symA := "TST" + uuid.NewString()[:6]
	symB := "TST" + uuid.NewString()[:6]
	// Watchlist writes now emit sync frames into outbox_events (FK → users), so both users must exist.
	mustExec(t, ctx, pool, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid,$2,now())`, userID, "wl-"+userID+"@test.local")
	mustExec(t, ctx, pool, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid,$2,now())`, otherUser, "wl-"+otherUser+"@test.local")
	mustExec(t, ctx, pool, `INSERT INTO instruments (instrument_id, symbol, name) VALUES ($1::uuid,$2,'A')`, instA, symA)
	mustExec(t, ctx, pool, `INSERT INTO instruments (instrument_id, symbol, name) VALUES ($1::uuid,$2,'B')`, instB, symB)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE user_id = ANY($1::uuid[])`, []string{userID, otherUser})
		_, _ = pool.Exec(ctx, `DELETE FROM watchlists WHERE user_id = ANY($1::uuid[])`, []string{userID, otherUser})
		_, _ = pool.Exec(ctx, `DELETE FROM instruments WHERE instrument_id = ANY($1::uuid[])`, []string{instA, instB})
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{userID, otherUser})
	})

	svc := watchlist.NewService(repo.NewWatchlistPostgres(pool))

	// Two named lists.
	tech, err := svc.CreateWatchlist(ctx, userID, "Tech")
	if err != nil {
		t.Fatalf("create Tech: %v", err)
	}
	shorts, err := svc.CreateWatchlist(ctx, userID, "Shorts")
	if err != nil {
		t.Fatalf("create Shorts: %v", err)
	}

	// The SAME instrument in BOTH lists (proves the multi-list PK).
	if err := svc.AddItem(ctx, userID, tech.ID, instA); err != nil {
		t.Fatalf("add A to Tech: %v", err)
	}
	if err := svc.AddItem(ctx, userID, shorts.ID, instA); err != nil {
		t.Fatalf("add A to Shorts: %v", err)
	}
	if err := svc.AddItem(ctx, userID, tech.ID, instB); err != nil {
		t.Fatalf("add B to Tech: %v", err)
	}

	lists, err := svc.ListWatchlists(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("expected 2 lists, got %d", len(lists))
	}
	if lists[0].ID != tech.ID || lists[1].ID != shorts.ID {
		t.Fatalf("list order = [%s, %s], want creation order [%s, %s]", lists[0].ID, lists[1].ID, tech.ID, shorts.ID)
	}
	counts := map[string]int{}
	for _, l := range lists {
		counts[l.Name] = l.ItemCount
	}
	if counts["Tech"] != 2 || counts["Shorts"] != 1 {
		t.Fatalf("item counts wrong: %+v", counts)
	}

	// ResolveInstruments (the universe port) resolves a manual list to its members.
	resolved, err := svc.ResolveInstruments(ctx, userID, tech.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved %d instruments, want 2", len(resolved))
	}

	// Rename.
	if err := svc.RenameWatchlist(ctx, userID, shorts.ID, "Short Book"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Ownership: another user cannot rename/delete/see this user's list.
	if err := svc.RenameWatchlist(ctx, otherUser, tech.ID, "Hijack"); err != watchlist.ErrNotFound {
		t.Fatalf("cross-tenant rename = %v, want ErrWatchlistNotFound", err)
	}
	if err := svc.AddItem(ctx, otherUser, tech.ID, instB); err != nil {
		t.Fatalf("cross-tenant add errored (should be a silent no-op via WHERE EXISTS): %v", err)
	}
	if got, _ := svc.ListItems(ctx, userID, tech.ID); len(got) != 2 {
		t.Fatalf("cross-tenant add leaked into the list: now %d items", len(got))
	}
	if got, err := svc.ListItems(ctx, otherUser, tech.ID); err != nil || len(got) != 0 {
		t.Fatalf("cross-tenant list = (%d items, %v), want empty", len(got), err)
	}
	if err := svc.RemoveItem(ctx, otherUser, tech.ID, instA); err != nil {
		t.Fatalf("cross-tenant remove errored (should be a silent no-op): %v", err)
	}
	if got, _ := svc.ListItems(ctx, userID, tech.ID); len(got) != 2 {
		t.Fatalf("cross-tenant remove changed the list: now %d items", len(got))
	}
	if err := svc.DeleteWatchlist(ctx, otherUser, tech.ID); err != watchlist.ErrNotFound {
		t.Fatalf("cross-tenant delete = %v, want ErrWatchlistNotFound", err)
	}

	// Delete cascades items.
	if err := svc.DeleteWatchlist(ctx, userID, tech.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var itemCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM watchlist_items WHERE watchlist_id = $1::uuid`, tech.ID).Scan(&itemCount); err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if itemCount != 0 {
		t.Fatalf("delete did not cascade items: %d remain", itemCount)
	}

	// Default-list resolution: listID "" hits the user's (now single) default list.
	if _, err := svc.ListItems(ctx, userID, ""); err != nil {
		t.Fatalf("default list resolve: %v", err)
	}

	// A screener-kind list reserves the dynamic path.
	scr := watchlist.Watchlist{ID: uuid.NewString(), Name: "Momo", Kind: watchlist.Screener}
	if _, err := repo.NewWatchlistPostgres(pool).CreateWatchlist(ctx, scr, userID); err != nil {
		t.Fatalf("create screener: %v", err)
	}
	if _, err := svc.ResolveInstruments(ctx, userID, scr.ID); err != watchlist.ErrScreenerNotImplemented {
		t.Fatalf("screener resolve = %v, want ErrScreenerNotImplemented", err)
	}
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}
