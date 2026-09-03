package service

import (
	"context"
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/shardv2"
)

type resourceReleaseStub struct{ release ResourceRelease }

func (r resourceReleaseStub) ActiveResourceRelease(context.Context, string, string, BuildChannel) (ResourceRelease, error) {
	return r.release, nil
}

func TestShardResourceWorkspaceGrantAndSourceValidation(t *testing.T) {
	registry := NewEntityRegistry(stubEntityService{kind: "artifact", observable: true, entities: map[string]string{"artifact-allowed": "Allowed", "artifact-private": "Private"}})
	provider := NewWorkspaceResourceProvider(registry)
	source := []byte(`{"version":2,"intent":"Workspace view","resources":{"nodes":{"uri":"shard://self/resources/nodes","kind":"collection","meaning":"Declared nodes","schemaVersion":1,"schema":{"type":"object"},"source":{"provider":"workspace.nodes","params":{"ids":["artifact-allowed"]}},"operations":["snapshot"],"observe":{"mode":"changes","protocol":"shard-data/1"},"exposure":{"app":["snapshot","observe"]}}},"bindings":{"nodes":{"resource":"nodes","params":{"ids":{"input":"ids"}},"inputsSchema":{"type":"object","properties":{"ids":{"type":"array","items":{"type":"string"}}},"required":["ids"],"additionalProperties":false}}}}`)
	compiled, err := shardv2.Compile(source, shardv2.Registry{"workspace.nodes": provider.Profile()})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewShardResourceService(bridgeStubArtifacts{id: "artifact-shard", typ: "app"}, resourceReleaseStub{ResourceRelease{Source: source, Hash: compiled.Hash, BuildID: "build1", Generation: "gen1"}}, map[string]ResourceProvider{"workspace.nodes": provider}, ResourceServiceOptions{})
	ctx := WithPrincipal(context.Background(), Principal{UserID: "owner", ActorType: ActorTypeUserSession})
	target := ResourceTarget{ShardID: "artifact-shard", Environment: ChannelPublished, Audience: "app", ContractHash: compiled.Hash}
	page, err := svc.Read(ctx, target, ResourceRequest{Binding: "nodes", Inputs: map[string]any{"ids": []any{"artifact-allowed"}}})
	if err != nil || len(page.Records) != 1 {
		t.Fatalf("allowed source: %+v %v", page, err)
	}
	var data map[string]any
	_ = json.Unmarshal(page.Records[0].Data, &data)
	if data["title"] != "Allowed" {
		t.Fatalf("wrong source data: %+v", data)
	}
	_, err = svc.Read(ctx, target, ResourceRequest{Binding: "nodes", Inputs: map[string]any{"ids": []any{"artifact-private"}}})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("parameter escape: %v", err)
	}
	_, err = svc.Mutate(ctx, target, ResourceMutation{ResourceRequest: ResourceRequest{Binding: "nodes", Inputs: map[string]any{"ids": []any{"artifact-allowed"}}, ID: "artifact-allowed"}, Op: "delete", BaseRevision: "0", RequestID: "delete1"})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("external mutation accepted: %v", err)
	}
	_, err = svc.Read(context.Background(), target, ResourceRequest{Binding: "nodes"})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("anonymous access: %v", err)
	}
}
