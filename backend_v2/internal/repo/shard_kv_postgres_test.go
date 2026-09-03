package repo

import (
	"aladin/backend_v2/internal/workspacesync"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	coreservice "aladin/backend_v2/internal/service"
)

// Shard-KV DB integration tests (sandbox only — see mustTestPool). Cover the
// revision-guard, tombstone semantics, prefix list, quota accounting, and the
// published-only frame emission + snapshot (SHARD_LOCAL_STATE.md).

func seedShardArtifact(ctx context.Context, t *testing.T, r *ShardKVRepo, id string) {
	t.Helper()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content, created_at, updated_at)
		VALUES ($1, $2::uuid, 'app', 'kv test shard', '', now(), now())
		ON CONFLICT (id) DO NOTHING
	`, id, testAdminUserID)
	if err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

func countShardKVFrames(ctx context.Context, t *testing.T, r *ShardKVRepo, entityID string) int {
	t.Helper()
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM outbox_events
		 WHERE user_id = $1::uuid AND type = 'data_event'
		   AND payload::text LIKE '%' || $2 || '%'
	`, testAdminUserID, entityID).Scan(&n)
	if err != nil {
		t.Fatalf("count frames: %v", err)
	}
	return n
}

func TestShardKV_GuardedWritesTombstonesAndFrames(t *testing.T) {
	ctx := context.Background()
	pool := mustTestPool(ctx, t)
	defer pool.Close()
	seedUser(ctx, t, pool, testAdminUserID)
	r := NewShardKVPostgres(pool)
	shard := tid("artifact-kv")
	seedShardArtifact(ctx, t, r, shard)
	val := func(s string) json.RawMessage { return json.RawMessage(s) }

	// New key: base 0 → revision 1, frame emitted (published).
	e, err := r.Set(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", val(`{"q":"a"}`), 0)
	if err != nil {
		t.Fatalf("Set new: %v", err)
	}
	if e.Revision != 1 {
		t.Fatalf("new key revision = %d, want 1", e.Revision)
	}
	if n := countShardKVFrames(ctx, t, r, shardKVEntityID(shard, "filters")); n != 1 {
		t.Errorf("frames after first set = %d, want 1", n)
	}

	// Stale base → conflict carrying current.
	_, err = r.Set(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", val(`{"q":"stale"}`), 0)
	var conflict *coreservice.ShardKVConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("stale Set err = %v, want ShardKVConflict", err)
	}
	if conflict.Current.Revision != 1 || !strings.Contains(string(conflict.Current.Value), `"a"`) {
		t.Fatalf("conflict current = %+v", conflict.Current)
	}

	// Correct base → revision 2.
	if e, err = r.Set(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", val(`{"q":"b"}`), 1); err != nil || e.Revision != 2 {
		t.Fatalf("guarded Set: rev=%d err=%v, want rev 2", e.Revision, err)
	}

	// New key with wrong base (row absent, base 5) must NOT create silently.
	if _, err = r.Set(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "ghost", val(`1`), 5); !errors.As(err, &conflict) {
		t.Fatalf("absent-row wrong-base Set err = %v, want conflict", err)
	} else if conflict.Current.Revision != 0 {
		t.Fatalf("absent-row conflict revision = %d, want 0", conflict.Current.Revision)
	}

	// Delete guarded: wrong base → conflict; right base → tombstone (revision 3).
	if err = r.Delete(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", 1); !errors.As(err, &conflict) {
		t.Fatalf("stale Delete err = %v, want conflict", err)
	}
	if err = r.Delete(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", 2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := r.Get(ctx, shard, coreservice.ChannelPublished, "filters"); ok {
		t.Fatalf("Get after delete should miss")
	}
	// Idempotent re-delete (already tombstoned; any base).
	if err = r.Delete(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", 99); err != nil {
		t.Fatalf("re-Delete should be idempotent: %v", err)
	}

	// Revive: set against the tombstone's revision (3).
	if e, err = r.Set(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "filters", val(`{"q":"c"}`), 3); err != nil || e.Revision != 4 || e.Deleted {
		t.Fatalf("revive Set: %+v err=%v, want rev 4 live", e, err)
	}

	// Frames so far for this key: set, set, delete, set = 4 (stale attempts emit nothing).
	if n := countShardKVFrames(ctx, t, r, shardKVEntityID(shard, "filters")); n != 4 {
		t.Errorf("frames = %d, want 4", n)
	}
}

func TestShardKV_DraftChannelIsSandboxed(t *testing.T) {
	ctx := context.Background()
	pool := mustTestPool(ctx, t)
	defer pool.Close()
	seedUser(ctx, t, pool, testAdminUserID)
	r := NewShardKVPostgres(pool)
	shard := tid("artifact-kv-draft")
	seedShardArtifact(ctx, t, r, shard)

	if _, err := r.Set(ctx, testAdminUserID, shard, coreservice.ChannelDraft, "scratch", json.RawMessage(`{"x":1}`), 0); err != nil {
		t.Fatalf("draft Set: %v", err)
	}
	// Draft writes emit NO frames (they never reach the replica)...
	if n := countShardKVFrames(ctx, t, r, shardKVEntityID(shard, "scratch")); n != 0 {
		t.Errorf("draft frames = %d, want 0", n)
	}
	// ...and stay channel-isolated from published reads.
	if _, ok, _ := r.Get(ctx, shard, coreservice.ChannelPublished, "scratch"); ok {
		t.Fatalf("draft row visible on published channel")
	}
}

func TestShardKV_ListPrefixQuotaSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := mustTestPool(ctx, t)
	defer pool.Close()
	seedUser(ctx, t, pool, testAdminUserID)
	r := NewShardKVPostgres(pool)
	shard := tid("artifact-kv-list")
	seedShardArtifact(ctx, t, r, shard)

	for _, k := range []string{"scenario/base", "scenario/stress", "annotations/aapl", "settings"} {
		if _, err := r.Set(ctx, testAdminUserID, shard, coreservice.ChannelPublished, k, json.RawMessage(`{"v":"`+k+`"}`), 0); err != nil {
			t.Fatalf("Set %s: %v", k, err)
		}
	}
	if err := r.Delete(ctx, testAdminUserID, shard, coreservice.ChannelPublished, "annotations/aapl", 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Prefix list: only scenario/*, ordered, tombstones excluded from full list.
	got, err := r.List(ctx, shard, coreservice.ChannelPublished, "scenario/")
	if err != nil || len(got) != 2 || got[0].Key != "scenario/base" || got[1].Key != "scenario/stress" {
		t.Fatalf("List(scenario/) = %+v err=%v", got, err)
	}
	all, err := r.List(ctx, shard, coreservice.ChannelPublished, "")
	if err != nil || len(all) != 3 {
		t.Fatalf("List(all) = %d entries err=%v, want 3 (tombstone hidden)", len(all), err)
	}

	used, err := r.UsedBytes(ctx, shard, coreservice.ChannelPublished)
	if err != nil || used <= 0 {
		t.Fatalf("UsedBytes = %d err=%v", used, err)
	}

	// Snapshot: published rows for this user's shards, tombstone INCLUDED as Op:delete.
	src := NewShardKVSyncSource(pool)
	if src.EntityKind() != "shard_kv" {
		t.Fatalf("EntityKind = %s", src.EntityKind())
	}
	snap, err := src.Snapshot(ctx, testAdminUserID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	var live, dead int
	for _, ent := range snap {
		if !strings.HasPrefix(ent.EntityID, shard+"#") {
			continue // rows from other tests' shards under this user
		}
		switch ent.Op {
		case workspacesync.OpUpsert:
			live++
			var d lightShardKVData
			if err := json.Unmarshal(ent.Data, &d); err != nil || d.ShardID != shard || d.Revision < 1 {
				t.Fatalf("bad upsert data: %s err=%v", ent.Data, err)
			}
		case workspacesync.OpDelete:
			dead++
			if ent.Seq < 2 {
				t.Fatalf("tombstone seq = %d, want bumped >= 2", ent.Seq)
			}
		}
	}
	if live != 3 || dead != 1 {
		t.Fatalf("snapshot live=%d dead=%d, want 3/1", live, dead)
	}
}
