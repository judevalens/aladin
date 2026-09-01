package repo

import (
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"encoding/json"
	"testing"
)

func TestShardReleaseAtomicCodeAndContract(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	publicationCount := func() int {
		t.Helper()
		var count int
		if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM outbox_events WHERE user_id=$1::uuid AND type='app_event' AND payload->>'resourceId'=$2 AND payload->>'operation'='published'`, testAdminUserID, h.target.ShardID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	beforePublications := publicationCount()
	releases := service.NewShardReleaseService(h.repo, h.profiles)
	build := service.BuildResult{OK: true, Contract: h.compiled.Source, Files: map[string][]byte{"bundle.js": []byte("console.log('first')"), "anchors.json": []byte(`{"version":1,"anchors":[]}`)}}
	build.BuildID = service.ShardBuildIdentity(build.Contract, build.Files)
	if err := releases.Stage(h.ctx, h.target.ShardID, service.ChannelPublished, build); err != nil {
		t.Fatal(err)
	}
	active, err := h.repo.ActiveResourceRelease(h.ctx, testAdminUserID, h.target.ShardID, service.ChannelPublished)
	if err != nil || active.BuildID != "build-1" {
		t.Fatalf("stage changed release: %+v %v", active, err)
	}
	if publicationCount() != beforePublications {
		t.Fatal("staging announced a publication")
	}
	if err := releases.Activate(h.ctx, h.target.ShardID, service.ChannelPublished, build.BuildID); err != nil {
		t.Fatal(err)
	}
	got, err := releases.Active(h.ctx, h.target.ShardID, service.ChannelPublished)
	if err != nil || string(got.Files["bundle.js"]) != "console.log('first')" || got.Hash != h.compiled.Hash {
		t.Fatalf("code/contract mismatch: %+v %v", got, err)
	}
	if publicationCount() != beforePublications+1 {
		t.Fatal("committed publication did not notify viewers")
	}
	var event struct {
		PageID       string `json:"page_id"`
		Protocol     string `json:"protocol"`
		BuildID      string `json:"buildId"`
		ContractHash string `json:"contractHash"`
	}
	var payload []byte
	if err := h.pool.QueryRow(h.ctx, `SELECT payload->'payload' FROM outbox_events WHERE user_id=$1::uuid AND type='app_event' AND payload->>'resourceId'=$2 AND payload->>'operation'='published' AND payload->'payload'->>'buildId'=$3`, testAdminUserID, h.target.ShardID, build.BuildID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &event); err != nil || event.PageID != h.target.ShardID || event.Protocol != "bridge/2" || event.BuildID != got.BuildID || event.ContractHash != got.Hash {
		t.Fatalf("publication event does not identify the active release: %+v %v", event, err)
	}
	// New code and a revoked agent grant stage together; failed activation must
	// preserve both previous code bytes and previous capabilities.
	var contract shardv2.Contract
	_ = json.Unmarshal(build.Contract, &contract)
	tasks := contract.Resources["tasks"]
	tasks.SchemaVersion++
	contract.Resources["tasks"] = tasks
	incompatible := build
	incompatible.Contract, _ = json.Marshal(contract)
	incompatible.Files = map[string][]byte{"bundle.js": []byte("console.log('second')")}
	incompatible.BuildID = service.ShardBuildIdentity(incompatible.Contract, incompatible.Files)
	if err := releases.Stage(h.ctx, h.target.ShardID, service.ChannelPublished, incompatible); err != nil {
		t.Fatal(err)
	}
	if err := releases.Activate(h.ctx, h.target.ShardID, service.ChannelPublished, incompatible.BuildID); err == nil {
		t.Fatal("incompatible schema activated")
	}
	if publicationCount() != beforePublications+1 {
		t.Fatal("failed activation announced a publication")
	}
	got, err = releases.Active(h.ctx, h.target.ShardID, service.ChannelPublished)
	if err != nil || got.BuildID != build.BuildID || string(got.Files["bundle.js"]) != "console.log('first')" {
		t.Fatalf("failed activation changed serving release: %+v %v", got, err)
	}
	// Pointer switch is idempotent after a lost publish response.
	if err := releases.Activate(h.ctx, h.target.ShardID, service.ChannelPublished, build.BuildID); err != nil {
		t.Fatal(err)
	}
	tampered := build
	tampered.Files = map[string][]byte{"bundle.js": []byte("tampered")}
	if err := releases.Stage(h.ctx, h.target.ShardID, service.ChannelPublished, tampered); err == nil {
		t.Fatal("forged build identity accepted")
	}
}

func TestShardReleaseRetentionAdmissionPreservesActiveBuild(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{Builds: 2})
	releases := service.NewShardReleaseService(h.repo, h.profiles)
	build := service.BuildResult{OK: true, Contract: h.compiled.Source, Files: map[string][]byte{"bundle.js": []byte("first")}}
	build.BuildID = service.ShardBuildIdentity(build.Contract, build.Files)
	if err := releases.Stage(h.ctx, h.target.ShardID, service.ChannelPublished, build); err != nil {
		t.Fatal(err)
	}
	if err := releases.Activate(h.ctx, h.target.ShardID, service.ChannelPublished, build.BuildID); err != nil {
		t.Fatal(err)
	}
	if err := releases.Stage(h.ctx, h.target.ShardID, service.ChannelPublished, build); err != nil {
		t.Fatalf("idempotent stage consumed quota: %v", err)
	}
	build.Files = map[string][]byte{"bundle.js": []byte("second")}
	build.BuildID = service.ShardBuildIdentity(build.Contract, build.Files)
	if service.ResourceErrorCode(releases.Stage(h.ctx, h.target.ShardID, service.ChannelPublished, build)) != "quota" {
		t.Fatal("stage quota not enforced")
	}
	active, err := releases.Active(h.ctx, h.target.ShardID, service.ChannelPublished)
	if err != nil || string(active.Files["bundle.js"]) != "first" {
		t.Fatalf("quota rejection changed active build: %+v %v", active, err)
	}
	var count int
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM shard_resource_releases WHERE shard_id=$1 AND environment='published'`, h.target.ShardID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("rejected stage left orphan: %d %v", count, err)
	}
}

func TestShardCatalogCurrentPublishedAuthorization(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	catalog := service.NewShardCatalogService(h.repo, h.svc)
	entries, err := catalog.Find(h.ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.ShardID == h.target.ShardID {
			found = true
			if entry.ContractHash != h.compiled.Hash {
				t.Fatal("stale catalog hash")
			}
		}
	}
	if !found {
		t.Fatal("published resources absent")
	}
	_, err = h.pool.Exec(h.ctx, `DELETE FROM artifacts WHERE id=$1`, h.target.ShardID)
	if err != nil {
		t.Fatal(err)
	}
	entries, err = catalog.Find(h.ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ShardID == h.target.ShardID {
			t.Fatal("deleted shard remains discoverable")
		}
	}
}
