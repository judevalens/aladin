package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type resourceAPIArtifacts struct {
	service.ArtifactService
	pool *pgxpool.Pool
}

func (a resourceAPIArtifacts) Get(ctx context.Context, id string) (service.ArtifactResponse, error) {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return service.ArtifactResponse{}, err
	}
	var result service.ArtifactResponse
	err = a.pool.QueryRow(ctx, `SELECT id,type FROM artifacts WHERE id=$1 AND user_id=$2::uuid`, id, p.UserID).Scan(&result.ID, &result.Type)
	if err != nil {
		return result, service.ErrNotFound
	}
	return result, nil
}

type resourceAPIAuth struct {
	fakeAuthService
	userID string
}

func (a *resourceAPIAuth) ResolveBearerToken(_ context.Context, token string) (service.Principal, error) {
	if token != "resource-test-token" {
		return service.Principal{}, service.ErrUnauthenticated
	}
	return service.Principal{UserID: a.userID, ActorType: service.ActorTypeUserSession, ActorID: a.userID}, nil
}

func TestShardResourceHTTPAndWebSocketWithPostgres(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("sandbox database unavailable: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	userID, shardID := uuid.NewString(), "artifact-"+uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,created_at,updated_at) VALUES($1::uuid,$2,now(),now())`, userID, userID+"@example.test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO artifacts(id,user_id,type,title,content,created_at,updated_at) VALUES($1,$2::uuid,'app','resource test','',now(),now())`, shardID, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, table := range []string{"shard_resource_cursors", "shard_resource_receipts", "shard_resource_records", "shard_resource_active", "shard_resource_releases"} {
			_, _ = pool.Exec(context.Background(), "DELETE FROM "+table+" WHERE user_id=$1::uuid AND shard_id=$2", userID, shardID)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM artifacts WHERE id=$1`, shardID)
	}()
	principal := service.WithPrincipal(ctx, service.Principal{UserID: userID, ActorType: service.ActorTypeUserSession, ActorID: userID})
	storage := repo.NewShardResourcePostgres(pool, repo.ShardResourceLimits{})
	profiles := shardv2.Registry{"shard.documents": storage.Profile()}
	source, err := os.ReadFile("../../../shared/shard-v2/fixtures/backend-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := shardv2.Compile(source, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.StageResourceRelease(principal, shardID, service.ChannelPublished, "test-build", "test-generation", compiled); err != nil {
		t.Fatal(err)
	}
	if err := storage.ActivateResourceRelease(principal, shardID, service.ChannelPublished, "test-build", profiles); err != nil {
		t.Fatal(err)
	}
	resources := service.NewShardResourceService(resourceAPIArtifacts{pool: pool}, storage, map[string]service.ResourceProvider{"shard.documents": storage}, service.ResourceServiceOptions{RefreshInterval: 10 * time.Millisecond})
	server := NewWithDependencies(":0", testDependencies{AuthSvc: &resourceAPIAuth{userID: userID}, ShardResourceSvc: resources})
	httpServer := httptest.NewServer(server.httpServer.Handler)
	defer httpServer.Close()
	path := "/api/shards/" + shardID + "/v2/published"
	call := func(method string, params map[string]any, hash string) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"aladin": "bridge/2", "type": "request", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+path+"/request", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer resource-test-token")
		req.Header.Set(shardContractHeader, hash)
		response, err := httpServer.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var body map[string]any
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if err := shardv2.ValidateProtocol("bridge-response", body); err != nil {
			t.Fatalf("invalid HTTP envelope: %v %+v", err, body)
		}
		return response.StatusCode, body
	}
	status, hello := call("hello", map[string]any{}, "")
	if status != 200 || hello["data"].(map[string]any)["contractHash"] != compiled.Hash {
		t.Fatalf("hello: %+v", hello)
	}
	_, response := call("resource.describe", map[string]any{"binding": "tasks"}, compiled.Hash)
	if err := shardv2.ValidateProtocol("descriptor", response["data"]); err != nil {
		t.Fatal(err)
	}
	conn, _, err := websocket.Dial(ctx, websocketURL(httpServer.URL, path+"/ws?contractHash="+compiled.Hash), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer resource-test-token"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err := wsjson.Write(ctx, conn, map[string]any{"aladin": "bridge/2", "type": "request", "id": 2, "method": "resource.subscribe", "params": map[string]any{"binding": "tasks"}}); err != nil {
		t.Fatal(err)
	}
	var ack map[string]any
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	if err := shardv2.ValidateProtocol("bridge-response", ack); err != nil {
		t.Fatal(err)
	}
	if err := shardv2.ValidateProtocol("subscription", ack["data"]); err != nil {
		t.Fatal(err)
	}
	var push map[string]any
	if err := wsjson.Read(ctx, conn, &push); err != nil {
		t.Fatal(err)
	}
	initial := push["data"].(map[string]any)
	if initial["seq"] != "0" || len(initial["records"].([]any)) != 0 {
		t.Fatalf("initial snapshot: %+v", push)
	}
	status, response = call("resource.insert", map[string]any{"binding": "tasks", "requestId": "http-insert", "data": map[string]any{"title": "persisted over HTTP"}}, compiled.Hash)
	if status != 200 {
		t.Fatalf("insert: %+v", response)
	}
	if err := wsjson.Read(ctx, conn, &push); err != nil {
		t.Fatal(err)
	}
	event := push["data"].(map[string]any)
	if err := shardv2.ValidateProtocol("event", event); err != nil {
		t.Fatal(err)
	}
	if event["seq"] != "1" || len(event["records"].([]any)) != 1 {
		t.Fatalf("live update: %+v", push)
	}
	status, response = call("resource.read", map[string]any{"binding": "tasks", "environment": "draft"}, compiled.Hash)
	if status != 400 || response["code"] != "bad-request" {
		t.Fatalf("authority escape: %+v", response)
	}
	status, response = call("resource.read", map[string]any{"binding": "tasks"}, "old-hash")
	if status != 409 || response["code"] != "contract-changed" {
		t.Fatalf("stale release: %+v", response)
	}
	identity := ack["data"].(map[string]any)
	if err := wsjson.Write(ctx, conn, map[string]any{"aladin": "bridge/2", "type": "request", "id": 3, "method": "resource.unsubscribe", "params": map[string]any{"subscriptionId": identity["subscriptionId"]}}); err != nil {
		t.Fatal(err)
	}
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack["id"] != float64(3) || ack["ok"] != true {
		t.Fatalf("unsubscribe: %+v", ack)
	}
}

func TestShardResourceRoutesDenyContentCredentialsAndDefaultOff(t *testing.T) {
	server := NewWithDependencies(":0", testDependencies{AuthSvc: &fakeAuthService{}})
	for _, path := range []string{"/api/shards/artifact-test/v2/published/request", "/api/shards/artifact-test/v2/published/ws?access_token=content-valid"} {
		method := http.MethodPost
		if path[len(path)-5:] == "valid" {
			method = http.MethodGet
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer content-valid")
		rec := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("content credential accepted: %d", rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/shards/artifact-test/v2/published/request", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer desktop-valid")
	rec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("v2 must default off: %d", rec.Code)
	}
}
