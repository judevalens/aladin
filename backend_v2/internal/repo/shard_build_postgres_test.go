package repo

import (
	"context"
	"testing"
	"time"

	coreservice "aladin/backend_v2/internal/service"
)

// SetStatus upserts the build state, round-trips through GetStatus, and emits an
// "artifact.build-status" app_event over the outbox that the drain surfaces —
// while NEVER leaking into the durable data-sync pull (the isolation guarantee
// that keeps ephemeral UI events out of the client's offline store).
func TestShardBuild_SetStatusEmitsAppEventIsolatedFromPull(t *testing.T) {
	ctx := adminContext(testAdminUserID)
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool := mustTestPool(ctxTO, t)
	defer pool.Close()
	cleanupSyncTables(ctxTO, t, pool)
	seedUser(ctxTO, t, pool, testAdminUserID)

	repo := NewShardBuildPostgres(pool)
	// Namespaced per test process: shard_build_state is keyed by page id, and this test used to
	// `DELETE FROM shard_build_state` globally — which wipes rows other packages' parallel runs
	// depend on. A unique id makes the global delete unnecessary.
	pageID := tid("artifact-build")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM shard_build_state WHERE page_id = $1`, pageID)
	})

	// Cursor BEFORE the writes, so the drain assertion below reads a bounded window containing
	// only this test's events (DrainSince is global across users and batch-limited).
	sync := NewSyncPostgres(pool)
	beforeCursor, err := sync.Horizon(ctxTO)
	if err != nil {
		t.Fatalf("horizon: %v", err)
	}

	// building → ok transition.
	if err := repo.SetStatus(ctx, coreservice.ShardBuildState{
		PageID: pageID, Channel: coreservice.ChannelDraft, Status: coreservice.ShardBuildBuilding,
	}); err != nil {
		t.Fatalf("SetStatus building: %v", err)
	}
	if err := repo.SetStatus(ctx, coreservice.ShardBuildState{
		PageID: pageID, Channel: coreservice.ChannelDraft, Status: coreservice.ShardBuildOK,
		BuildID: "abc123", BuiltAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SetStatus ok: %v", err)
	}

	// GetStatus reflects the latest (upserted) state.
	got, err := repo.GetStatus(ctx, pageID, coreservice.ChannelDraft)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if got.Status != coreservice.ShardBuildOK || got.BuildID != "abc123" {
		t.Fatalf("GetStatus = %+v, want ok/abc123", got)
	}
	if got.BuiltAt == "" {
		t.Error("GetStatus built_at should be set on ok")
	}

	// Unknown page/channel → zero status, not an error.
	none, err := repo.GetStatus(ctx, "artifact-missing", coreservice.ChannelPublished)
	if err != nil {
		t.Fatalf("GetStatus(missing): %v", err)
	}
	if none.Status != "" {
		t.Errorf("missing GetStatus = %q, want empty status", none.Status)
	}

	// ISOLATION: the durable pull must NOT surface app_event rows — otherwise the
	// client's offline data engine would try to apply build-status as an entity.
	frames, _, err := sync.PullSince(ctxTO, testAdminUserID, 0)
	if err != nil {
		t.Fatalf("PullSince: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("PullSince surfaced %d frames; app_events must be invisible to the data stream", len(frames))
	}

	// The drain DOES surface them, as AppEvents carrying the build-status target. Count only THIS
	// test's events: the drain is global across users, so other packages' app_events (notifications,
	// market quotes) legitimately share the window — asserting on the global total was the reason
	// this test flaked.
	events, _, err := sync.DrainSince(ctxTO, beforeCursor)
	if err != nil {
		t.Fatalf("DrainSince: %v", err)
	}
	var appEvents int
	for _, e := range events {
		if e.AppEvent == nil || e.AppEvent.ResourceID != pageID {
			continue
		}
		appEvents++
		if e.AppEvent.ResourceKind != "artifact" || e.AppEvent.Operation != "build-status" {
			t.Errorf("app event target = %+v, want artifact/build-status/%s", e.AppEvent, pageID)
		}
	}
	if appEvents != 2 { // building + ok
		t.Errorf("drain surfaced %d app events for %s, want 2 (building + ok)", appEvents, pageID)
	}
}
