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
	if _, err := pool.Exec(ctxTO, `DELETE FROM shard_build_state`); err != nil {
		t.Fatalf("cleanup shard_build_state: %v", err)
	}
	seedUser(ctxTO, t, pool, testAdminUserID)

	repo := NewShardBuildPostgres(pool)
	const pageID = "artifact-build-1"

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

	sync := NewSyncPostgres(pool)

	// ISOLATION: the durable pull must NOT surface app_event rows — otherwise the
	// client's offline data engine would try to apply build-status as an entity.
	frames, _, err := sync.PullSince(ctxTO, testAdminUserID, 0)
	if err != nil {
		t.Fatalf("PullSince: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("PullSince surfaced %d frames; app_events must be invisible to the data stream", len(frames))
	}

	// The drain DOES surface them, as AppEvents carrying the build-status target.
	events, _, err := sync.DrainSince(ctxTO, 0)
	if err != nil {
		t.Fatalf("DrainSince: %v", err)
	}
	var appEvents int
	for _, e := range events {
		if e.AppEvent == nil {
			continue
		}
		appEvents++
		if e.AppEvent.ResourceKind != "artifact" || e.AppEvent.Operation != "build-status" || e.AppEvent.ResourceID != pageID {
			t.Errorf("app event target = %+v, want artifact/build-status/%s", e.AppEvent, pageID)
		}
	}
	if appEvents != 2 { // building + ok
		t.Errorf("drain surfaced %d app events, want 2 (building + ok)", appEvents)
	}
}
