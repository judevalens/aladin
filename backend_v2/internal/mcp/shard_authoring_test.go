package mcpserver

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"
	shardstorage "aladin/backend_v2/internal/shardresource/storage"
	"aladin/backend_v2/internal/shardv2"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type authoringReleaseStub struct {
	service.ShardReleaseService
	enabled bool
	active  *service.ShardRelease
	err     error
}

func (r authoringReleaseStub) Enabled() bool { return r.enabled }
func (r authoringReleaseStub) Active(context.Context, string, service.BuildChannel) (service.ShardRelease, error) {
	if r.err != nil {
		return service.ShardRelease{}, r.err
	}
	if r.active != nil {
		return *r.active, nil
	}
	return service.ShardRelease{}, service.ErrNotFound
}

func TestShardAuthoringInstructionsUseCapabilityDiscovery(t *testing.T) {
	if !strings.Contains(mcpInstructions, "get_authoring_guide") || !strings.Contains(mcpInstructions, "page_id") {
		t.Fatal("MCP clients must discover both new and existing shard capabilities")
	}
	for _, staleAPI := range []string{"useKV", "useShardState", "useNode", "useResource", "bridge/1", "bridge/2"} {
		if strings.Contains(mcpInstructions, staleAPI) {
			t.Fatalf("static instructions duplicate capability-specific guidance: %s", staleAPI)
		}
	}
}

func TestRuntimeAuthoringGuideContainsRunnableGraphQLShape(t *testing.T) {
	for _, want := range []string{
		`"schema": "graphql/schema.graphql"`,
		`"Query.taskSummary"`,
		`"capabilities": ["tasks:query"]`,
		`defineResolver`,
		`ctx.capabilities.call("tasks:query"`,
		`executeGraphQL`,
		`from "@aladin/shard"`,
	} {
		if !strings.Contains(runtimeAuthoringGuide, want) {
			t.Fatalf("runtime guide missing %q", want)
		}
	}
	if strings.Contains(runtimeAuthoringGuide, "subscribeResource") || strings.Contains(runtimeAuthoringGuide, "@aladin/kit") {
		t.Fatal("runtime guide advertises an unavailable client API")
	}
}

// Exercise tool calls over MCP, including the returned contract, rather than
// testing only string helpers. No DB is needed for guide selection or creation.
func TestShardAuthoringEnabledCapabilitiesAndCreation(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "key-value"
		if enabled {
			name = "resources"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			store := docsurface.NewStore(root)
			ctx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)
			artifacts := &fakeArtifactService{getResult: service.ArtifactResponse{ID: "page-created", Type: "app"}}
			profiles := shardv2.Registry(nil)
			if enabled {
				profiles = shardv2.Registry{"shard.documents": shardstorage.NewShardResourcePostgres(nil, shardstorage.ShardResourceLimits{}).Profile(), "workspace.nodes": service.NewWorkspaceResourceProvider(nil).Profile()}
			}
			releases := authoringReleaseStub{enabled: service.NewShardReleaseService(nil, profiles).Enabled()}
			server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "authoring", Version: "test"}, nil)
			registerDocSurfaceTools(server, artifacts, store, nil, nil, nil, releases)
			ct, st := sdkmcp.NewInMemoryTransports()
			ss, err := server.Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "test"}, nil)
			cs, err := client.Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			call := func(name string, arguments map[string]any) map[string]any {
				t.Helper()
				result, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: arguments})
				if err != nil || result.IsError {
					t.Fatalf("%s: %+v %v", name, result, err)
				}
				value, ok := result.StructuredContent.(map[string]any)
				if !ok {
					t.Fatalf("%s output: %T", name, result.StructuredContent)
				}
				return value
			}
			guide := call("get_authoring_guide", map[string]any{})["authoring_guide"].(string)
			for _, want := range []string{"ordinary React", "Tailwind utilities as the primary styling system", "data-anchor", "@aladin/shard"} {
				if !strings.Contains(guide, want) {
					t.Fatalf("authoring guide missing %q", want)
				}
			}
			for _, removed := range []string{"@aladin/kit exports", "<AppShell", "<DataTable", "prefer kit primitives"} {
				if strings.Contains(guide, removed) {
					t.Fatalf("authoring guide still advertises removed UI %q", removed)
				}
			}
			if strings.Contains(guide, "useResource(") != enabled || strings.Contains(guide, "useKV(") == enabled {
				t.Fatalf("guide advertises wrong data API: %s", guide)
			}
			for _, forbidden := range []string{"Shard v2", "V1 ", "V2 ", "if enabled", "only when the backend", "bridge/1", "bridge/2"} {
				if strings.Contains(guide, forbidden) {
					t.Fatalf("conditional/version guidance leaked: %s", forbidden)
				}
			}
			created := call("create_app", map[string]any{"title": "Capability-selected app"})
			if created["authoring_guide"] != guide {
				t.Fatal("create and discovery disagree")
			}
			id := created["id"].(string)
			starter := created["current_index_tsx"].(string)
			if strings.Contains(starter, "@aladin/kit") || !strings.Contains(starter, "data-anchor=\"intro\"") || !strings.Contains(starter, "font-display") {
				t.Fatalf("starter does not model custom token-backed UI: %s", starter)
			}
			contract, err := store.ReadFile(ctx, id, "contract.json")
			if enabled {
				if err != nil || created["contract_json"] != string(contract) {
					t.Fatalf("contract not returned/persisted: %v", err)
				}
				compiled, err := shardv2.Compile(contract, profiles)
				if err != nil {
					t.Fatalf("seeded contract invalid: %v", err)
				}
				if len(compiled.Contract.Resources["settings"].Exposure.Agent) != 0 {
					t.Fatal("new app granted agent access implicitly")
				}
			} else if !errors.Is(err, service.ErrNotFound) || created["contract_json"] != nil {
				t.Fatalf("disabled resource configuration seeded: %v", err)
			}
			selected := call("get_authoring_guide", map[string]any{"page_id": id})
			if selected["authoring_guide"] != guide {
				t.Fatal("existing target no longer matches its create result")
			}
			runtime := docsurface.NewBuilder(store, filepath.Join(root, "cache", "esm"), profiles)
			built, err := runtime.Build(ctx, id, service.ChannelDraft)
			if err != nil || !built.OK {
				t.Fatalf("new app does not build: %v %s", err, built.Log)
			}
			if (len(built.Contract) > 0) != enabled {
				t.Fatal("build runtime disagrees with guide")
			}
		})
	}
}

func TestShardAuthoringPreservesExistingStorage(t *testing.T) {
	for _, tc := range []struct {
		name          string
		enabled       bool
		contract      bool
		protected     bool
		blocked       bool
		wantResources bool
	}{
		{name: "existing key-value on enabled backend", enabled: true},
		{name: "existing resources", enabled: true, contract: true, wantResources: true},
		{name: "resource execution disabled", contract: true, blocked: true},
		{name: "missing contract with protected release", enabled: true, protected: true, wantResources: true},
		{name: "missing contract with disabled protected release", protected: true, blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := docsurface.NewStore(root)
			ctx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)
			id := "artifact-existing"
			_, err := store.EnsurePageDir(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.WriteFile(ctx, id, "index.tsx", []byte("original source")); err != nil {
				t.Fatal(err)
			}
			if tc.contract {
				if err := store.WriteFile(ctx, id, "contract.json", []byte(starterResourceContractJSON)); err != nil {
					t.Fatal(err)
				}
			}
			releases := authoringReleaseStub{enabled: tc.enabled}
			if tc.protected {
				if tc.enabled {
					releases.active = &service.ShardRelease{ResourceRelease: service.ResourceRelease{Source: []byte(starterResourceContractJSON)}}
				} else {
					releases.err = service.ResourceFailure("unsupported-capability", "disabled")
				}
			}
			tools := docToolServer{artifacts: fakeArtifacts{}, store: store, releases: releases}
			_, got, err := tools.getAuthoringGuide(ctx, nil, authoringGuideInput{PageID: id})
			if err != nil {
				t.Fatal(err)
			}
			if tc.blocked {
				if !strings.Contains(got.AuthoringGuide, "unavailable") || strings.Contains(got.AuthoringGuide, "useResource(") || strings.Contains(got.AuthoringGuide, "useKV(") {
					t.Fatal("disabled target advertised executable data API")
				}
			} else if strings.Contains(got.AuthoringGuide, "useResource(") != tc.wantResources || strings.Contains(got.AuthoringGuide, "useKV(") == tc.wantResources {
				t.Fatalf("target's API was silently changed: %s", got.AuthoringGuide)
			}
			if got.IndexTSX != "original source" {
				t.Fatal("existing source changed")
			}
			_, err = store.ReadFile(ctx, id, "contract.json")
			if !tc.contract && !errors.Is(err, service.ErrNotFound) {
				t.Fatal("guide wrote a contract/migrated an existing shard")
			}
		})
	}
}

type failedAuthoringStore struct{ service.DocSurfaceStore }

func (failedAuthoringStore) ReadFile(context.Context, string, string) ([]byte, error) {
	return nil, errors.New("store unavailable")
}
func TestShardAuthoringDoesNotGuessAfterReadFailure(t *testing.T) {
	tools := docToolServer{artifacts: fakeArtifacts{}, store: failedAuthoringStore{}, releases: authoringReleaseStub{enabled: true}}
	_, _, err := tools.getAuthoringGuide(context.Background(), nil, authoringGuideInput{PageID: "artifact-test"})
	if err == nil || err.Error() != "store unavailable" {
		t.Fatalf("read failure masked by a guessed guide: %v", err)
	}
}
