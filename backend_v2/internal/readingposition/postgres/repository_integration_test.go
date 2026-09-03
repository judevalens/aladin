package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/readingposition"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reading-position DB integration tests (sandbox only — see mustTestPool).
// Cover the LWW upsert + seq bump, ownership check, frame emission, and the
// cold-start snapshot.

var testUserID = uuid.NewString()
var testSuffix = uuid.NewString()[:8]

func testID(name string) string { return name + "-" + testSuffix }

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, dbtest.RequireTestDSN(t))
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

func seedUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,created_at,updated_at) VALUES ($1::uuid,$2,now(),now()) ON CONFLICT (id) DO NOTHING`, testUserID, testUserID+"@reading-position.test"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedReadingArtifact(ctx context.Context, t *testing.T, r *Repository, id string) {
	t.Helper()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, created_at, updated_at)
		VALUES ($1, $2::uuid, 'file', 'reading test doc', '', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, id, testUserID)
	if err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

func countReadingPositionFrames(ctx context.Context, t *testing.T, r *Repository, artifactID string) int {
	t.Helper()
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM outbox_events
		 WHERE user_id = $1::uuid AND type = 'data_event'
		   AND payload::text LIKE '%reading_position%'
		   AND payload::text LIKE '%' || $2 || '%'
	`, testUserID, artifactID).Scan(&n)
	if err != nil {
		t.Fatalf("count frames: %v", err)
	}
	return n
}

func TestReadingPosition_PutBumpsSeqAndEmitsFrames(t *testing.T) {
	ctx := context.Background()
	pool := mustTestPool(ctx, t)
	defer pool.Close()
	seedUser(ctx, t, pool)
	r := New(pool)
	doc := testID("artifact-readpos")
	seedReadingArtifact(ctx, t, r, doc)

	// First report: seq 1.
	p, err := r.PutReadingPosition(ctx, testUserID, doc, 12)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if p.Page != 12 || p.Seq != 1 {
		t.Fatalf("first put = %+v, want page 12 seq 1", p)
	}
	if p.UpdatedAt <= 0 {
		t.Fatalf("first put updatedAt = %d, want unix ms", p.UpdatedAt)
	}

	// LWW: a later report simply wins; seq bumps.
	if p, err = r.PutReadingPosition(ctx, testUserID, doc, 87); err != nil || p.Page != 87 || p.Seq != 2 {
		t.Fatalf("second put = %+v err=%v, want page 87 seq 2", p, err)
	}
	if n := countReadingPositionFrames(ctx, t, r, doc); n != 2 {
		t.Errorf("frames = %d, want 2", n)
	}

	// Read back.
	got, ok, err := r.GetReadingPosition(ctx, testUserID, doc)
	if err != nil || !ok || got.Page != 87 {
		t.Fatalf("Get = %+v ok=%v err=%v, want page 87", got, ok, err)
	}

	// Unknown artifact (or another user's) is rejected, nothing emitted.
	if _, err := r.PutReadingPosition(ctx, testUserID, testID("artifact-readpos-missing"), 3); !errors.Is(err, readingposition.ErrReadingPositionNotFound) {
		t.Fatalf("put on missing artifact err = %v, want ErrReadingPositionNotFound", err)
	}

	// Absent position reads as not-found, no error.
	if _, ok, err := r.GetReadingPosition(ctx, testUserID, testID("artifact-readpos-none")); err != nil || ok {
		t.Fatalf("get absent = ok=%v err=%v, want ok=false", ok, err)
	}
}

func TestReadingPosition_SnapshotCarriesTheRow(t *testing.T) {
	ctx := context.Background()
	pool := mustTestPool(ctx, t)
	defer pool.Close()
	seedUser(ctx, t, pool)
	r := New(pool)
	doc := testID("artifact-readpos-snap")
	seedReadingArtifact(ctx, t, r, doc)
	if _, err := r.PutReadingPosition(ctx, testUserID, doc, 42); err != nil {
		t.Fatalf("Put: %v", err)
	}

	ents, err := NewSyncSource(pool).Snapshot(ctx, testUserID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	for _, ent := range ents {
		if ent.EntityID != doc {
			continue
		}
		if ent.EntityKind != "reading_position" || ent.Op != coreservice.OpUpsert || ent.Seq != 1 {
			t.Fatalf("snapshot entity = %+v", ent)
		}
		var d struct {
			ArtifactID string `json:"artifactId"`
			Page       int64  `json:"page"`
			UpdatedAt  int64  `json:"updatedAt"`
		}
		if err := json.Unmarshal(ent.Data, &d); err != nil {
			t.Fatalf("snapshot data: %v", err)
		}
		if d.ArtifactID != doc || d.Page != 42 || d.UpdatedAt <= 0 {
			t.Fatalf("snapshot data = %+v", d)
		}
		return
	}
	t.Fatalf("snapshot missing %s (got %d entities)", doc, len(ents))
}
