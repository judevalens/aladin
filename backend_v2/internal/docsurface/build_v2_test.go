package docsurface

import (
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildV2CapturesImmutableOutputs(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	ctx := testCtx()
	_, _ = store.EnsurePageDir(ctx, "shard")
	source, err := os.ReadFile("../../../shared/shard-v2/fixtures/backend-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	profiles := shardv2.Registry{"shard.documents": {Version: 1, Owned: true, Operations: []string{"snapshot", "query", "insert", "update", "delete"}, Observation: "refresh-snapshots", ParamsSchema: shardv2.Schema{"type": "object", "additionalProperties": false}}}
	for name, data := range map[string][]byte{"contract.json": source, "anchors.json": []byte(`{"version":1,"intent":"Test","anchors":[{"id":"tasks","route":"#/","meaning":"Tasks","binding":{"id":"tasks"}}]}`), "index.tsx": []byte(`document.getElementById("root").textContent = "first";`)} {
		if err := store.WriteFile(ctx, "shard", name, data); err != nil {
			t.Fatal(err)
		}
	}
	disabled := NewBuilder(store, filepath.Join(root, "cache", "esm"))
	res, err := disabled.Build(ctx, "shard", service.ChannelPublished)
	if err != nil || res.OK || !strings.Contains(res.Log, "disabled") {
		t.Fatalf("v2 bypassed feature flag: %+v %v", res, err)
	}
	runtime := NewBuilder(store, filepath.Join(root, "cache", "esm"), profiles)
	first, err := runtime.Build(ctx, "shard", service.ChannelPublished)
	if err != nil || !first.OK {
		t.Fatalf("build: %+v %v", first, err)
	}
	if first.BuildID != service.ShardBuildIdentity(first.Contract, first.Files) {
		t.Fatal("unbound build identity")
	}
	if _, err := store.ReadFile(ctx, "shard", "dist/bundle.js"); err == nil {
		t.Fatal("v2 wrote mutable serving dist")
	}
	if err := store.WriteFile(ctx, "shard", "index.tsx", []byte(`document.getElementById("root").textContent = "second";`)); err != nil {
		t.Fatal(err)
	}
	second, err := runtime.Build(ctx, "shard", service.ChannelPublished)
	if err != nil || !second.OK || second.BuildID == first.BuildID {
		t.Fatalf("new code not identified: %+v %v", second, err)
	}
	if !strings.Contains(string(first.Files["bundle.js"]), "first") {
		t.Fatal("prior build bytes changed")
	}
	if err := store.WriteFile(ctx, "shard", "contract.json", []byte(`{"version":2}`)); err != nil {
		t.Fatal(err)
	}
	invalid, err := runtime.Build(ctx, "shard", service.ChannelPublished)
	if err != nil || invalid.OK || !strings.Contains(invalid.Log, "contract.json") {
		t.Fatalf("invalid contract accepted: %+v %v", invalid, err)
	}
}
