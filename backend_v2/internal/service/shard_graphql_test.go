package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type graphqlResourceStub struct {
	request  ResourceRequest
	target   ResourceTarget
	mutation ResourceMutation
}

func (*graphqlResourceStub) Hello(context.Context, ResourceTarget) (map[string]any, error) {
	return nil, nil
}
func (*graphqlResourceStub) Describe(context.Context, ResourceTarget, ResourceRequest) (ResourceDescriptor, error) {
	return ResourceDescriptor{}, nil
}
func (s *graphqlResourceStub) Read(_ context.Context, target ResourceTarget, request ResourceRequest) (ResourceSnapshot, error) {
	s.target, s.request = target, request
	return ResourceSnapshot{Complete: true}, nil
}
func (s *graphqlResourceStub) Mutate(_ context.Context, target ResourceTarget, mutation ResourceMutation) (ResourceMutationResult, error) {
	s.target, s.mutation = target, mutation
	return ResourceMutationResult{RequestID: mutation.RequestID}, nil
}
func (*graphqlResourceStub) Subscribe(context.Context, ResourceTarget, ResourceRequest) (ResourceSubscription, error) {
	return ResourceSubscription{}, nil
}

type graphqlReleaseStub struct{ release ShardRelease }

func (*graphqlReleaseStub) Enabled() bool { return true }
func (*graphqlReleaseStub) Stage(context.Context, string, BuildChannel, BuildResult) error {
	return nil
}
func (*graphqlReleaseStub) Activate(context.Context, string, BuildChannel, string) error { return nil }
func (s *graphqlReleaseStub) Active(context.Context, string, BuildChannel) (ShardRelease, error) {
	return s.release, nil
}

func graphqlTestReleases() ShardReleaseService {
	source := json.RawMessage(`{"version":2,"intent":"test","resources":{"tasks":{"uri":"shard://self/resources/tasks","kind":"collection","meaning":"tasks","schemaVersion":1,"schema":{"type":"object"},"source":{"provider":"shard.documents","dataset":"tasks"},"operations":["snapshot","query","update"]}},"bindings":{"tasks":{"resource":"tasks"}},"graphql":{"schema":"graphql/schema.graphql","operations":{},"resolvers":{"Query.tasks":{"file":"resolvers/tasks.ts","export":"default","capabilities":["tasks:query","tasks:update"],"budget":{"maxOperations":2,"maxDocuments":10,"timeoutMs":1000,"memoryMiB":32}}}},"lambdas":{}}`)
	return &graphqlReleaseStub{release: ShardRelease{
		ResourceRelease: ResourceRelease{Source: source, Hash: "release", BuildID: "build", Generation: "initial"},
		Files: map[string][]byte{
			"resolver.bundle.mjs":   []byte("export const resolvers = {}; export const lambdas = {}"),
			"runtime-manifest.json": []byte(`{"graphql":{"operations":{},"resolvers":{}},"lambdas":{}}`),
			"schema.graphql":        []byte("type Query { ok: Boolean! }"),
		},
	}}
}

func TestShardGraphQLPassesCapabilityScopePerExecution(t *testing.T) {
	requests := map[string]map[string]any{}
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests[r.URL.Path] = body
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer runtime.Close()
	serviceValue := NewShardGraphQLService(graphqlTestReleases(), &graphqlResourceStub{}, runtime.URL, "01234567890123456789012345678901")
	ctx := WithPrincipal(context.Background(), Principal{UserID: "owner", ActorType: ActorTypeUserSession, Scopes: []string{ScopeArtifactsRead}})
	_, err := serviceValue.Execute(ctx, ResourceTarget{ShardID: "shard", Environment: ChannelDraft, Audience: "agent", ContractHash: "release"}, ShardGraphQLRequest{OperationID: "summary"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := requests["/v1/releases/prepare"]["scopeToken"]; ok {
		t.Fatal("prepare request retained an audience-scoped capability token")
	}
	token, ok := requests["/v1/graphql/execute"]["scopeToken"].(string)
	if !ok || token == "" {
		t.Fatal("execute request did not carry its capability scope")
	}
	scope, err := serviceValue.(*shardGraphQLService).verifyScope(token)
	if err != nil || scope.Audience != "agent" || scope.ShardID != "shard" {
		t.Fatalf("execution scope = %+v, %v", scope, err)
	}
}

func TestShardRuntimeFailureCodeTranslation(t *testing.T) {
	for _, test := range []struct{ runtime, resource string }{
		{"FORBIDDEN", "forbidden"},
		{"NOT_FOUND", "not-found"},
		{"RELEASE_CHANGED", "contract-changed"},
		{"TIMEOUT", "source-unavailable"},
		{"RUNTIME_ERROR", "invalid-schema"},
	} {
		t.Run(test.runtime, func(t *testing.T) {
			runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": test.runtime, "error": "failed"})
			}))
			defer runtime.Close()
			serviceValue := NewShardGraphQLService(graphqlTestReleases(), &graphqlResourceStub{}, runtime.URL, "01234567890123456789012345678901").(*shardGraphQLService)
			_, err := serviceValue.call(context.Background(), "/failure", map[string]any{})
			if ResourceErrorCode(err) != test.resource {
				t.Fatalf("call() error = %v, want %s", err, test.resource)
			}
		})
	}
}

func TestShardRuntimeCapabilityIsSignedReleaseAndBindingScoped(t *testing.T) {
	resources := &graphqlResourceStub{}
	serviceValue := NewShardGraphQLService(graphqlTestReleases(), resources, "http://runtime", "01234567890123456789012345678901").(*shardGraphQLService)
	scope := runtimeScope{UserID: "owner", ShardID: "shard", Environment: ChannelDraft, ReleaseHash: "release", Audience: "app", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	token, err := serviceValue.signScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	result, err := serviceValue.Capability(context.Background(), "Bearer 01234567890123456789012345678901", RuntimeCapabilityRequest{ReleaseHash: "release", Handler: "Query.tasks", Capability: "tasks:query", ScopeToken: token, Input: map[string]any{"query": map[string]any{"limit": 4}}})
	if err != nil || result.(ResourceSnapshot).Complete != true {
		t.Fatalf("capability: %+v %v", result, err)
	}
	if resources.request.Binding != "tasks" || resources.request.Query == nil || resources.request.Query.Limit != 4 {
		t.Fatalf("unscoped request: %+v", resources.request)
	}
	if resources.target.ShardID != "shard" || resources.target.ContractHash != "release" {
		t.Fatalf("untrusted target: %+v", resources.target)
	}
	_, err = serviceValue.Capability(context.Background(), "Bearer 01234567890123456789012345678901", RuntimeCapabilityRequest{ReleaseHash: "release", Handler: "Query.tasks", Capability: "tasks:delete", ScopeToken: token})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("undeclared capability accepted: %v", err)
	}
	_, err = serviceValue.Capability(context.Background(), "Bearer 01234567890123456789012345678901", RuntimeCapabilityRequest{ReleaseHash: "other", Capability: "tasks:query", ScopeToken: token})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("release forgery accepted: %v", err)
	}
	forged := token[:len(token)-1] + "x"
	_, err = serviceValue.Capability(context.Background(), "Bearer 01234567890123456789012345678901", RuntimeCapabilityRequest{ReleaseHash: "release", Capability: "tasks:query", ScopeToken: forged})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("signature forgery accepted: %v", err)
	}
	_, err = serviceValue.Capability(context.Background(), "01234567890123456789012345678901", RuntimeCapabilityRequest{ReleaseHash: "release", Handler: "Query.tasks", Capability: "tasks:query", ScopeToken: token})
	if ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("raw shared secret accepted without bearer scheme: %v", err)
	}
}

func TestShardRuntimeCapabilityMapsMutationWithoutNamespaceInput(t *testing.T) {
	resources := &graphqlResourceStub{}
	serviceValue := NewShardGraphQLService(graphqlTestReleases(), resources, "http://runtime", "01234567890123456789012345678901").(*shardGraphQLService)
	token, _ := serviceValue.signScope(runtimeScope{UserID: "owner", ShardID: "shard", Environment: ChannelPublished, ReleaseHash: "release", Audience: "agent", ExpiresAt: time.Now().Add(time.Minute).Unix()})
	data, _ := json.Marshal(map[string]any{"title": "task"})
	_, err := serviceValue.Capability(context.Background(), "Bearer 01234567890123456789012345678901", RuntimeCapabilityRequest{ReleaseHash: "release", Handler: "Query.tasks", Capability: "tasks:update", ScopeToken: token, Input: map[string]any{"id": "a", "baseRevision": "1", "requestId": "r", "data": json.RawMessage(data), "namespace": "other"}})
	if err != nil {
		t.Fatal(err)
	}
	if resources.mutation.Binding != "tasks" || resources.mutation.Op != "update" || resources.mutation.ID != "a" || resources.target.ShardID != "shard" {
		t.Fatalf("mutation scope escaped: %+v %+v", resources.mutation, resources.target)
	}
}
