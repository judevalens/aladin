package mcpserver

import (
	"aladin/backend_v2/internal/alert"
	"aladin/backend_v2/internal/artifactref"
	"aladin/backend_v2/internal/copilot"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/api"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/document"
	"aladin/backend_v2/internal/feed"
	"aladin/backend_v2/internal/file"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/graphpane"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/page"
	"aladin/backend_v2/internal/providerconnection"
	"aladin/backend_v2/internal/readingposition"
	"aladin/backend_v2/internal/realtime"
	"aladin/backend_v2/internal/record"
	"aladin/backend_v2/internal/relationship"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/research"
	searchdomain "aladin/backend_v2/internal/search"
	"aladin/backend_v2/internal/service"
	shardstorage "aladin/backend_v2/internal/shardresource/storage"
	"aladin/backend_v2/internal/shardv2"
	"aladin/backend_v2/internal/source"
	"aladin/backend_v2/internal/system"
	"aladin/backend_v2/internal/unfurl"
	"aladin/backend_v2/internal/watchlist"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const resourcePilotTSX = `import {createRoot} from "react-dom/client";
import {useState,useEffect} from "react";
import {useResource,queryResource,resourceRequestId} from "@aladin/shard";
function App(){
 const [pagination,setPagination]=useState("");
 const tasks=useResource("tasks"), prefs=useResource("preferences"), live=useResource("workspace"), notes=useResource("notes");
 useEffect(()=>{window.parent.postMessage({type:"pilot.render",text:document.getElementById("tasks")?.textContent},"*");},[tasks]);
 return <main data-anchor="main" data-kind="collection"><div id="tasks">{tasks.status}:{JSON.stringify(tasks.records)}</div>
 <div id="live">{live.status}:{JSON.stringify(live.records)}{live.error?.message}</div>
 <div id="third">{notes.status}:{JSON.stringify(notes.records)}</div>
 <div id="prefs">{prefs.status}:{JSON.stringify(prefs.records)}:{prefs.error?.message}</div>
 <button id="create" disabled={!tasks.insert} onClick={()=>tasks.insert({requestId:resourceRequestId(),data:{title:"Created in draft"}})}>Create</button>
 <button id="pref" disabled={!prefs.insert} onClick={()=>prefs.insert({requestId:resourceRequestId(),data:{theme:"light"}})}>Preference</button>
 <button id="note" disabled={!notes.insert} onClick={()=>notes.insert({requestId:resourceRequestId(),data:{title:"Third resource"}})}>Note</button>
 <button id="extra" disabled={!tasks.insert} onClick={()=>tasks.insert({requestId:resourceRequestId(),data:{title:"Second draft task"}})}>Second task</button>
 <button id="paginate" onClick={async()=>{try{
   const query={limit:1,orderBy:[{field:"/title",direction:"asc"}]};
   const first=await queryResource("tasks",query);
   if(!first.nextCursor)throw new Error("missing cursor");
   const second=await queryResource("tasks",{...query,cursor:first.nextCursor});
   setPagination("pagination:"+(first.records.length+second.records.length)+":"+(first.records[0].id!==second.records[0].id));
 }catch(error){setPagination("query failed:"+String(error?.message||error)+":"+JSON.stringify(error));}}}>Page</button>
 <div id="pagination">{pagination}</div>
 </main>;
}createRoot(document.getElementById("root")).render(<App/>);`

type pilotAuth struct {
	service.AuthService
	principal service.Principal
}

func (a pilotAuth) ResolveBearerToken(_ context.Context, token string) (service.Principal, error) {
	if token == "pilot-content-only" {
		return service.Principal{UserID: a.principal.UserID, ActorType: service.ActorTypeContentToken, ActorID: "content", Scopes: []string{service.ScopeContentRead}}, nil
	}
	if token != "pilot-test-token" {
		return service.Principal{}, service.ErrUnauthenticated
	}
	return a.principal, nil
}

// Runs on the isolated test DB and real Chromium. Preview uses the same shard SDK,
// client and resource service, with actual draft persistence and workspace data.
func TestShardV2Pilot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbtest.RequireTestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	user, id := uuid.NewString(), "artifact-"+uuid.NewString()
	p := service.Principal{UserID: user, ActorType: service.ActorTypeUserSession, ActorID: user}
	ctx = service.WithPrincipal(ctx, p)
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,created_at,updated_at) VALUES($1::uuid,$2,now(),now())`, user, user+"@pilot.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO artifacts(id,user_id,type,title,content,created_at,updated_at) VALUES($1,$2::uuid,'app','Live source initial','',now(),now())`, id, user); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, table := range []string{"shard_resource_cursors", "shard_resource_receipts", "shard_resource_records", "shard_resource_active", "shard_resource_releases", "shard_build_state"} {
			_, _ = pool.Exec(context.Background(), "DELETE FROM "+table+" WHERE "+map[bool]string{true: "page_id", false: "shard_id"}[table == "shard_build_state"]+"=$1", id)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM artifacts WHERE id=$1`, id)
	}()
	artifacts := service.NewArtifactService(repo.NewArtifactsPostgres(pool), nil)
	storage := shardstorage.NewShardResourcePostgres(pool, shardstorage.ShardResourceLimits{})
	workspace := service.NewWorkspaceResourceProvider(service.NewEntityRegistry(service.NewArtifactEntityService(artifacts)))
	profiles := shardv2.Registry{"shard.documents": storage.Profile(), "workspace.nodes": workspace.Profile()}
	resources := service.NewShardResourceService(artifacts, storage, map[string]service.ResourceProvider{"shard.documents": storage, "workspace.nodes": workspace}, service.ResourceServiceOptions{RefreshInterval: 25 * time.Millisecond})
	releases := service.NewShardReleaseService(storage, profiles, workspace.(service.ResourceStageValidator))
	catalog := service.NewShardCatalogService(storage, resources)
	// Registration exercises SDK schema derivation too (including recursive query predicates).
	sdk := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "pilot", Version: "1"}, nil)
	registerShardResourceTools(sdk, resources, catalog, nil)
	root := t.TempDir()
	store := docsurface.NewStore(root)
	runtime := docsurface.NewBuilder(store, filepath.Join(root, "cache", "esm"), profiles)
	builds := service.NewShardBuildService(runtime, repo.NewShardBuildPostgres(pool), releases)
	preview := docsurface.NewPreviewSessions(store, runtime, docsurface.PreviewOptions{Builder: builds, Resources: resources, Releases: releases})
	defer preview.CloseAll(context.Background())
	source, err := os.ReadFile("../../../shared/shard-v2/fixtures/backend-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract shardv2.Contract
	if err := json.Unmarshal(source, &contract); err != nil {
		t.Fatal(err)
	}
	contract.Resources["workspace"] = shardv2.Resource{URI: "shard://self/resources/workspace", Kind: "collection", Meaning: "Live workspace source", SchemaVersion: 1, Schema: shardv2.Schema{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"}}, "required": []any{"id", "title"}, "additionalProperties": true}, Source: shardv2.Source{Provider: "workspace.nodes", Params: map[string]any{"ids": []any{id}}}, Operations: []string{"snapshot"}, Observe: &shardv2.Observation{Mode: "changes", Protocol: "shard-data/1"}, Exposure: shardv2.Exposure{App: []string{"snapshot", "observe"}, Agent: []string{"snapshot", "observe"}}}
	contract.Bindings["workspace"] = shardv2.Binding{Resource: "workspace"}
	notes := contract.Resources["tasks"]
	notes.URI = "shard://self/resources/notes"
	notes.Source.Dataset = "notes"
	notes.Meaning = "Third resource using the same provider"
	contract.Resources["notes"] = notes
	contract.Bindings["notes"] = shardv2.Binding{Resource: "notes"}
	source, _ = json.Marshal(contract)
	_, _ = store.EnsurePageDir(ctx, id)
	for name, data := range map[string][]byte{"index.tsx": []byte(resourcePilotTSX), "contract.json": source, "anchors.json": []byte(`{"version":1,"intent":"V2 pilot","anchors":[{"id":"main","route":"#/","meaning":"Pilot data","binding":{"id":"tasks"}}]}`)} {
		if err := store.WriteFile(ctx, id, name, data); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	state, err := preview.Open(ctx, id, service.ChannelDraft, service.PreviewOpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Mounted {
		t.Fatalf("shard did not mount: %+v", state)
	}
	wait := func(selector, want string) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			st, err := preview.Eval(ctx, id, "document.querySelector("+quotePilot(selector)+")?.textContent")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(st.EvalResult, want) {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		st, _ := preview.Snapshot(ctx, id)
		t.Fatalf("%s never contained %q: %+v", selector, want, st)
	}
	wait("#live", "Live source initial")
	wait("#tasks", "live")
	wait("#third", "live")
	wait("#prefs", "ready")
	t.Logf("first build + browser + four resource views: %s", time.Since(start))
	for _, button := range []string{"#create", "#pref", "#note"} {
		if _, err := preview.Click(ctx, id, button); err != nil {
			t.Fatal(err)
		}
	}
	wait("#tasks", "Created in draft")
	wait("#prefs", "light")
	wait("#third", "Third resource")
	if _, err := preview.Click(ctx, id, "#extra"); err != nil {
		t.Fatal(err)
	}
	wait("#tasks", "Second draft task")
	if _, err := preview.Click(ctx, id, "#paginate"); err != nil {
		t.Fatal(err)
	}
	wait("#pagination", "pagination:2:true")
	if _, err := pool.Exec(ctx, `UPDATE artifacts SET title='Live source changed',updated_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	wait("#live", "Live source changed")
	if err := preview.Close(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := preview.Open(ctx, id, service.ChannelDraft, service.PreviewOpenOptions{}); err != nil {
		t.Fatal(err)
	}
	wait("#tasks", "Created in draft")
	publisher := docToolServer{artifacts: artifacts, store: store, build: builds, preview: preview, releases: releases}
	_, published, err := publisher.publishApp(ctx, nil, publishAppInput{PageID: id})
	if err != nil || !published.Verified {
		t.Fatalf("publish: %+v %v", published, err)
	}
	if err := preview.Close(ctx, id); err != nil {
		t.Fatal(err)
	}
	agent := shardResourceTools{resources: resources, catalog: catalog}
	uri := "shard://" + id + "/resources/tasks"
	_, read, err := agent.read(ctx, nil, shardResourceInput{URI: uri})
	if err != nil || len(read.Snapshot.Records) != 0 {
		t.Fatalf("draft leaked into published: %+v %v", read, err)
	}
	_, created, err := agent.mutate(ctx, nil, mutateShardResourceInput{URI: uri, ContractHash: read.ContractHash, Op: "insert", RequestID: "pilot-insert", Data: map[string]any{"title": "Written with iframe closed"}})
	if err != nil {
		t.Fatal(err)
	}
	_, again, err := agent.mutate(ctx, nil, mutateShardResourceInput{URI: uri, ContractHash: read.ContractHash, Op: "insert", RequestID: "pilot-insert", Data: map[string]any{"title": "Written with iframe closed"}})
	if err != nil || again.Record.ID != created.Record.ID {
		t.Fatal("retry was not idempotent", err)
	}
	_, read, err = agent.read(ctx, nil, shardResourceInput{URI: uri})
	if err != nil || len(read.Snapshot.Records) != 1 {
		t.Fatalf("closed-iframe read: %+v %v", read, err)
	}
	_, _, err = agent.mutate(ctx, nil, mutateShardResourceInput{URI: "shard://" + id + "/resources/preferences", ContractHash: read.ContractHash, Op: "insert", RequestID: "forbidden", Data: map[string]any{"theme": "dark"}})
	if service.ResourceErrorCode(err) != "forbidden" {
		t.Fatalf("agent capability bypass: %v", err)
	}
	// Exercise MCP JSON-RPC input/output handling, not just direct Go handlers.
	ct, st := sdkmcp.NewInMemoryTransports()
	ss, err := sdk.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "pilot-client", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for name, args := range map[string]map[string]any{
		"find_shard_resources":    {"query": "", "limit": 10},
		"describe_shard_resource": {"uri": uri},
		"read_shard_resource":     {"uri": uri},
		"query_shard_resource":    {"uri": uri, "query": map[string]any{"limit": 1, "where": map[string]any{"and": []any{map[string]any{"field": "/title", "op": "eq", "value": "Written with iframe closed"}}}}},
		"mutate_shard_resource":   {"uri": uri, "contractHash": read.ContractHash, "op": "update", "id": created.Record.ID, "baseRevision": created.Record.Revision, "requestId": "mcp-wire-update", "data": map[string]any{"title": "Written with iframe closed"}},
	} {
		result, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError || result.StructuredContent == nil {
			t.Fatalf("MCP wire %s: %+v %v", name, result, err)
		}
	}
	apiServer := api.NewWithDependencies(":0", pilotAPIDependencies{AuthSvc: pilotAuth{principal: p}, ArtifactsSvc: artifacts, DocSurfaceStoreSvc: store, WorkspaceRuntimeSvc: runtime, ShardResourceSvc: resources, ShardReleaseSvc: releases})
	httpServer := httptest.NewServer(apiServer.Handler())
	defer httpServer.Close()
	active, err := releases.Active(ctx, id, service.ChannelPublished)
	if err != nil {
		t.Fatal(err)
	}
	// Mutable dist and draft authoring files cannot change served bytes or CSP.
	_ = store.WriteFile(ctx, id, "dist/bundle.js", []byte("MUTABLE_DIST_MUST_NOT_SERVE"))
	request, _ := http.NewRequestWithContext(ctx, "GET", httpServer.URL+"/content/"+id+"/?build_id="+active.BuildID, nil)
	request.Header.Set("Authorization", "Bearer pilot-test-token")
	response, err := httpServer.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || strings.Contains(string(body), "MUTABLE_DIST_MUST_NOT_SERVE") || !strings.Contains(string(body), active.Hash) || !strings.Contains(response.Header.Get("Content-Security-Policy"), "'unsafe-eval'") {
		t.Fatalf("protected serving failed: %d %s", response.StatusCode, body)
	}
	// Disabling execution must not expose an older mutable v1 build.
	disabledAPI := api.NewWithDependencies(":0", pilotAPIDependencies{AuthSvc: pilotAuth{principal: p}, ArtifactsSvc: artifacts, DocSurfaceStoreSvc: store, WorkspaceRuntimeSvc: runtime, ShardReleaseSvc: service.NewShardReleaseService(storage, nil)})
	disabledRequest := httptest.NewRequest("GET", "/content/"+id+"/", nil)
	disabledRequest.Header.Set("Authorization", "Bearer pilot-test-token")
	disabledResponse := httptest.NewRecorder()
	disabledAPI.Handler().ServeHTTP(disabledResponse, disabledRequest)
	if disabledResponse.Code != http.StatusServiceUnavailable || strings.Contains(disabledResponse.Body.String(), "MUTABLE_DIST_MUST_NOT_SERVE") {
		t.Fatalf("disabled v2 fell back to legacy content: %d", disabledResponse.Code)
	}
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+"/api/shard-resources/ws?access_token=pilot-test-token", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"tauri://localhost"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	for i, environment := range []string{"published", "draft"} {
		envelope := map[string]any{"target": map[string]any{"shardId": id, "environment": environment, "contractHash": active.Hash}, "request": map[string]any{"aladin": "bridge/2", "type": "request", "id": i + 1, "method": "resource.subscribe", "params": map[string]any{"binding": "tasks"}}}
		if err := wsjson.Write(ctx, ws, envelope); err != nil {
			t.Fatal(err)
		}
		var ack, push map[string]any
		if err := wsjson.Read(ctx, ws, &ack); err != nil {
			t.Fatal(err)
		}
		if ack["ok"] != true {
			t.Fatalf("shared socket ack: %+v", ack)
		}
		if err := wsjson.Read(ctx, ws, &push); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(push)
		want := "Written with iframe closed"
		if environment == "draft" {
			want = "Created in draft"
		}
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("environment routing: %s", encoded)
		}
	}
	exerciseShardWebHost(t, ctx, apiServer.Handler(), id, func() {
		_, _, err := agent.mutate(ctx, nil, mutateShardResourceInput{URI: uri, ContractHash: read.ContractHash, Op: "insert", RequestID: "web-host-visible", Data: map[string]any{"title": "Agent change in both web frames"}})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("complete persisted/live/third-resource/publish/MCP/shared-WS pilot: %s", time.Since(start))
}

type pilotAPIDependencies struct {
	emptyAPIDependencies
	AuthSvc             service.AuthService
	ArtifactsSvc        service.ArtifactService
	DocSurfaceStoreSvc  service.DocSurfaceStore
	WorkspaceRuntimeSvc service.WorkspaceRuntime
	ShardResourceSvc    service.ShardResourceService
	ShardReleaseSvc     service.ShardReleaseService
}

type emptyAPIDependencies struct{}

func (emptyAPIDependencies) Auth() service.AuthService          { return nil }
func (emptyAPIDependencies) System() system.SystemService       { return nil }
func (emptyAPIDependencies) Sources() source.SourceService      { return nil }
func (emptyAPIDependencies) Records() record.RecordService      { return nil }
func (emptyAPIDependencies) Artifacts() service.ArtifactService { return nil }
func (emptyAPIDependencies) Pages() page.Service                { return nil }
func (emptyAPIDependencies) Files() file.FileService            { return nil }
func (emptyAPIDependencies) Feed() feed.FeedService             { return nil }
func (emptyAPIDependencies) Insights() insights.InsightService  { return nil }
func (emptyAPIDependencies) ProviderConnections() providerconnection.ProviderConnectionService {
	return nil
}
func (emptyAPIDependencies) Realtime() realtime.EventService                 { return nil }
func (emptyAPIDependencies) RealtimeKeyResolver() realtime.KeyResolver       { return nil }
func (emptyAPIDependencies) Sync() service.SyncService                       { return nil }
func (emptyAPIDependencies) DocSurfaceStore() service.DocSurfaceStore        { return nil }
func (emptyAPIDependencies) WorkspaceRuntime() service.WorkspaceRuntime      { return nil }
func (emptyAPIDependencies) ShardBuild() service.ShardBuildService           { return nil }
func (emptyAPIDependencies) ShardResources() service.ShardResourceService    { return nil }
func (emptyAPIDependencies) ShardGraphQL() service.ShardGraphQLService       { return nil }
func (emptyAPIDependencies) ShardReleases() service.ShardReleaseService      { return nil }
func (emptyAPIDependencies) ShardKV() service.ShardKVService                 { return nil }
func (emptyAPIDependencies) ShardBridge() service.ShardBridgeService         { return nil }
func (emptyAPIDependencies) Relationships() relationship.RelationshipService { return nil }
func (emptyAPIDependencies) Research() research.ResearchService              { return nil }
func (emptyAPIDependencies) Documents() document.DocumentService             { return nil }
func (emptyAPIDependencies) GraphPane() graphpane.GraphPaneService           { return nil }
func (emptyAPIDependencies) EntityTags() service.EntityTagService            { return nil }
func (emptyAPIDependencies) ArtifactRefs() artifactref.ArtifactRefService    { return nil }
func (emptyAPIDependencies) EntityContext() service.EntityContextService     { return nil }
func (emptyAPIDependencies) EntityList() service.EntityListService           { return nil }
func (emptyAPIDependencies) Instruments() instrument.InstrumentService       { return nil }
func (emptyAPIDependencies) Watchlist() watchlist.Service                    { return nil }
func (emptyAPIDependencies) ReadingPositions() readingposition.Service       { return nil }
func (emptyAPIDependencies) Search() searchdomain.SearchService              { return nil }
func (emptyAPIDependencies) Unfurl() unfurl.UnfurlService                    { return nil }
func (emptyAPIDependencies) Bars() market.BarService                         { return nil }
func (emptyAPIDependencies) Alerts() alert.AlertService                      { return nil }
func (emptyAPIDependencies) Notifications() alert.NotificationService        { return nil }
func (emptyAPIDependencies) MarketData() market.MarketDataService            { return nil }
func (emptyAPIDependencies) GraphReader() graph.GraphReader                  { return nil }
func (emptyAPIDependencies) Copilot() copilot.CopilotService                 { return nil }

func (d pilotAPIDependencies) Auth() service.AuthService                { return d.AuthSvc }
func (d pilotAPIDependencies) Artifacts() service.ArtifactService       { return d.ArtifactsSvc }
func (d pilotAPIDependencies) DocSurfaceStore() service.DocSurfaceStore { return d.DocSurfaceStoreSvc }
func (d pilotAPIDependencies) WorkspaceRuntime() service.WorkspaceRuntime {
	return d.WorkspaceRuntimeSvc
}
func (d pilotAPIDependencies) ShardResources() service.ShardResourceService {
	return d.ShardResourceSvc
}
func (d pilotAPIDependencies) ShardReleases() service.ShardReleaseService { return d.ShardReleaseSvc }

var _ api.Dependencies = pilotAPIDependencies{}

func quotePilot(value string) string { raw, _ := json.Marshal(value); return string(raw) }
