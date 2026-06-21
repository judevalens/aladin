package db

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestClaimBatch_ReclaimsStaleHeartbeat asserts the crash-recovery backstop: a stream stuck in
// 'syncing' whose heartbeat has gone stale is re-claimed, while one with a fresh heartbeat is left
// alone (its worker is still alive). Both are NOT due for a normal refresh, so only the reclaim
// path can pick them.
func TestClaimBatch_ReclaimsStaleHeartbeat(t *testing.T) {
	t.Parallel()
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := uuid.NewString()[:8]
	stale := "stale-" + tag
	fresh := "fresh-" + tag

	// Both: poll + active + 'syncing' + recently refreshed (NOT due). Differ only by heartbeat age.
	for _, s := range []struct {
		key string
		hb  string // heartbeat age expression
	}{
		{stale, "now() - interval '5 minutes'"}, // worker crashed mid-fetch → reclaimable
		{fresh, "now()"},                        // worker alive → left alone
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO provider_streams (provider, stream_kind, stream_key, name, sync_mode, sync_state, sync_status, last_refresh_at, last_heartbeat_at)
			VALUES ('bluesky','search',$1,$1,'poll','active','syncing', now(), `+s.hb+`)
		`, s.key); err != nil {
			t.Fatalf("seed %s: %v", s.key, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM provider_streams WHERE stream_key IN ($1,$2)`, stale, fresh)
	})

	claimed, err := NewProviderStreamRepository(pool).ClaimBatch(ctx, 50)
	if err != nil {
		t.Fatalf("ClaimBatch: %v", err)
	}
	got := map[string]bool{}
	for _, c := range claimed {
		got[c.StreamKey] = true
	}
	if !got[stale] {
		t.Fatalf("a stream with a stale heartbeat must be reclaimed; claimed keys = %v", got)
	}
	if got[fresh] {
		t.Fatalf("a stream with a fresh heartbeat must NOT be reclaimed (its worker is alive)")
	}
}
