package repo_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// lightWatchlistData mirrors the wire shape the repo emits (test-local copy).
type lightWatchlistData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Position int64  `json:"position"`
	Items    []struct {
		InstrumentID string `json:"instrumentId"`
		Symbol       string `json:"symbol"`
		Name         string `json:"name"`
	} `json:"items"`
}

// TestWatchlistSyncFrames proves watchlists are on the R1 sync spine: create/add/rename emit
// upsert frames whose data carries the ordered items[]; the per-list seq is monotonic; delete
// emits a tombstone; and the cold-start Snapshot reflects the same (including the tombstone).
func TestWatchlistSyncFrames(t *testing.T) {
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
	instA := uuid.NewString()
	symA := "TST" + uuid.NewString()[:6]
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $1 || '@test.local', now())`, userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO instruments (instrument_id, symbol, name) VALUES ($1::uuid,$2,'Alpha Co')`, instA, symA); err != nil {
		t.Fatalf("seed instrument: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM outbox_events WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM watchlists WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM instruments WHERE instrument_id = $1::uuid`, instA)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, userID)
	})

	r := repo.NewWatchlistPostgres(pool)
	src := repo.NewWatchlistSyncSource(pool)

	list, err := r.CreateWatchlist(ctx, coreservice.Watchlist{ID: uuid.NewString(), Name: "Semis"}, userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.AddItem(ctx, userID, list.ID, instA); err != nil {
		t.Fatalf("add: %v", err)
	}

	// The durable outbox holds the emitted frames (data_event rows, newest last).
	var frameCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE user_id=$1::uuid AND type='data_event'`, userID).Scan(&frameCount); err != nil {
		t.Fatalf("count frames: %v", err)
	}
	if frameCount < 2 {
		t.Fatalf("want >=2 data_event frames after create+add, got %d", frameCount)
	}

	// Snapshot reflects the live list with its item.
	ents, err := src.Snapshot(ctx, userID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ent := findEntity(t, ents, list.ID)
	if ent.Op != coreservice.OpUpsert {
		t.Fatalf("snapshot op = %s, want upsert", ent.Op)
	}
	if ent.Seq < 2 {
		t.Fatalf("seq after create+add = %d, want >=2 (monotonic)", ent.Seq)
	}
	var d lightWatchlistData
	if err := json.Unmarshal(ent.Data, &d); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if d.Name != "Semis" || len(d.Items) != 1 || d.Items[0].Symbol != symA {
		t.Fatalf("frame data wrong: %+v", d)
	}
	seqAfterAdd := ent.Seq

	// Rename bumps the same entity's seq.
	if err := r.RenameWatchlist(ctx, userID, list.ID, "Semiconductors"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	ents, _ = src.Snapshot(ctx, userID)
	ent = findEntity(t, ents, list.ID)
	if ent.Seq <= seqAfterAdd {
		t.Fatalf("rename did not bump seq: %d <= %d", ent.Seq, seqAfterAdd)
	}
	_ = json.Unmarshal(ent.Data, &d)
	if d.Name != "Semiconductors" {
		t.Fatalf("rename not reflected in frame: %q", d.Name)
	}

	// Delete emits a tombstone; the snapshot carries it as Op:delete (blocks resurrection).
	if err := r.DeleteWatchlist(ctx, userID, list.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	ents, _ = src.Snapshot(ctx, userID)
	ent = findEntity(t, ents, list.ID)
	if ent.Op != coreservice.OpDelete {
		t.Fatalf("post-delete snapshot op = %s, want delete", ent.Op)
	}
	if len(ent.Data) != 0 {
		t.Fatalf("tombstone should carry no data, got %s", ent.Data)
	}
}

func findEntity(t *testing.T, ents []coreservice.FrameEntity, id string) coreservice.FrameEntity {
	t.Helper()
	for _, e := range ents {
		if e.EntityID == id {
			return e
		}
	}
	t.Fatalf("entity %s not in snapshot (%d entities)", id, len(ents))
	return coreservice.FrameEntity{}
}
