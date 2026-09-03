package storage

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type resourceTestArtifacts struct {
	service.ArtifactService
	pool *pgxpool.Pool
}

func (a resourceTestArtifacts) Get(ctx context.Context, id string) (service.ArtifactResponse, error) {
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

type resourceHarness struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	repo     *ShardResourcePostgres
	svc      service.ShardResourceService
	target   service.ResourceTarget
	compiled *shardv2.Compiled
	profiles shardv2.Registry
}

var testAdminUserID = uuid.NewString()

func mustTestPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, dbtest.RequireTestDSN(t))
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func seedUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,created_at,updated_at) VALUES ($1::uuid,$2,now(),now()) ON CONFLICT (id) DO NOTHING`, userID, userID+"@shard-resource.test"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedShardArtifact(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO artifacts (id,user_id,type,title,content,created_at,updated_at) VALUES ($1,$2::uuid,'app','resource test shard','',now(),now()) ON CONFLICT (id) DO NOTHING`, id, testAdminUserID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

func setupResourceHarness(t *testing.T, limits ShardResourceLimits) *resourceHarness {
	t.Helper()
	ctx := service.WithPrincipal(context.Background(), service.Principal{UserID: testAdminUserID, ActorType: service.ActorTypeUserSession, ActorID: testAdminUserID})
	pool := mustTestPool(ctx, t)
	t.Cleanup(pool.Close)
	seedUser(ctx, t, pool, testAdminUserID)
	shard := "artifact-" + uuid.NewString()
	seedShardArtifact(ctx, t, pool, shard)
	r := NewShardResourcePostgres(pool, limits)
	profiles := shardv2.Registry{"shard.documents": r.Profile()}
	source, err := os.ReadFile("../../../shared/shard-v2/fixtures/backend-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := shardv2.Compile(source, profiles)
	if err != nil {
		t.Fatal(err)
	}
	target := service.ResourceTarget{ShardID: shard, Environment: service.ChannelPublished, Audience: "app", ContractHash: compiled.Hash}
	for _, environment := range []service.BuildChannel{service.ChannelDraft, service.ChannelPublished} {
		if err := r.StageResourceRelease(ctx, shard, environment, "build-1", "generation-1", compiled); err != nil {
			t.Fatal(err)
		}
		if err := r.ActivateResourceRelease(ctx, shard, environment, "build-1", profiles); err != nil {
			t.Fatal(err)
		}
	}
	svc := service.NewShardResourceService(resourceTestArtifacts{pool: pool}, r, map[string]service.ResourceProvider{"shard.documents": r}, service.ResourceServiceOptions{RefreshInterval: 10 * time.Millisecond})
	t.Cleanup(func() {
		for _, table := range []string{"shard_resource_cursors", "shard_resource_receipts", "shard_resource_records", "shard_resource_active", "shard_resource_releases"} {
			_, _ = pool.Exec(context.Background(), "DELETE FROM "+table+" WHERE user_id=$1::uuid AND shard_id=$2", testAdminUserID, shard)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM artifacts WHERE id=$1`, shard)
	})
	return &resourceHarness{ctx: ctx, pool: pool, repo: r, svc: svc, target: target, compiled: compiled, profiles: profiles}
}
func (h *resourceHarness) insert(t *testing.T, id, data string) service.ResourceMutationResult {
	t.Helper()
	result, err := h.svc.Mutate(h.ctx, h.target, service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: id}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(data)})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func (h *resourceHarness) read(t *testing.T, request service.ResourceRequest) service.ResourceSnapshot {
	t.Helper()
	result, err := h.svc.Read(h.ctx, h.target, request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func requireResourceCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || service.ResourceErrorCode(err) != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestShardResourcePersistenceAndIsolation(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	h.insert(t, "a", `{"title":"one","private":"secret","done":false}`)
	page := h.read(t, service.ResourceRequest{Binding: "publicTasks"})
	if string(page.Records[0].Data) != `{"title":"one"}` {
		t.Fatalf("projection leaked: %s", page.Records[0].Data)
	}
	draft := h.target
	draft.Environment = service.ChannelDraft
	page, err := h.svc.Read(h.ctx, draft, service.ResourceRequest{Binding: "tasks"})
	if err != nil || len(page.Records) != 0 {
		t.Fatalf("draft leak: %+v %v", page, err)
	}
	// Neither v1 REST list nor local replica can see v2 data.
	legacy, err := repo.NewShardKVPostgres(h.pool).List(h.ctx, h.target.ShardID, service.ChannelPublished, "")
	if err != nil || len(legacy) != 0 {
		t.Fatalf("v1 namespace leak: %v %v", legacy, err)
	}
	other := service.WithPrincipal(context.Background(), service.Principal{UserID: uuid.NewString(), ActorType: service.ActorTypeUserSession})
	_, err = h.svc.Read(other, h.target, service.ResourceRequest{Binding: "tasks"})
	requireResourceCode(t, err, "not-found")
	// Recreate service/repo: the data and active release do not depend on memory.
	fresh := NewShardResourcePostgres(h.pool, ShardResourceLimits{})
	svc := service.NewShardResourceService(resourceTestArtifacts{pool: h.pool}, fresh, map[string]service.ResourceProvider{"shard.documents": fresh}, service.ResourceServiceOptions{})
	page, err = svc.Read(h.ctx, h.target, service.ResourceRequest{Binding: "tasks"})
	if err != nil || len(page.Records) != 1 {
		t.Fatalf("restart lost data: %+v %v", page, err)
	}
}
func TestShardResourceConcurrentWritesAndDurableReceipts(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	h.insert(t, "a", `{"title":"before","private":"remove"}`)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.svc.Mutate(h.ctx, h.target, service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: "a"}, Op: "update", BaseRevision: "1", RequestID: uuid.NewString(), Data: json.RawMessage(`{"title":"after"}`)})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			requireResourceCode(t, err, "conflict")
		}
	}
	if success != 1 {
		t.Fatalf("concurrent CAS successes=%d", success)
	}
	page := h.read(t, service.ResourceRequest{Binding: "tasks"})
	if string(page.Records[0].Data) != `{"title":"after"}` {
		t.Fatalf("update merged data: %s", page.Records[0].Data)
	}
	command := service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks"}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(`{"title":"generated"}`)}
	first, err := h.svc.Mutate(h.ctx, h.target, command)
	if err != nil {
		t.Fatal(err)
	}
	fresh := NewShardResourcePostgres(h.pool, ShardResourceLimits{})
	restarted := service.NewShardResourceService(resourceTestArtifacts{pool: h.pool}, fresh, map[string]service.ResourceProvider{"shard.documents": fresh}, service.ResourceServiceOptions{})
	replay, err := restarted.Mutate(h.ctx, h.target, command)
	if err != nil || replay.Record.ID != first.Record.ID || replay.Record.Revision != first.Record.Revision {
		t.Fatalf("receipt not durable: %+v %v", replay, err)
	}
	command.Data = json.RawMessage(`{"title":"different"}`)
	_, err = restarted.Mutate(h.ctx, h.target, command)
	requireResourceCode(t, err, "conflict")
	if len(h.read(t, service.ResourceRequest{Binding: "tasks"}).Records) != 2 {
		t.Fatal("replay duplicated data")
	}
}
func TestShardResourceConcurrentInsertIDAndRequestID(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	command := service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: "same"}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(`{"title":"same"}`)}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := h.svc.Mutate(h.ctx, h.target, command); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(h.read(t, service.ResourceRequest{Binding: "tasks"}).Records) != 1 {
		t.Fatal("parallel replay duplicated records")
	}
	command.RequestID = uuid.NewString()
	_, err := h.svc.Mutate(h.ctx, h.target, command)
	requireResourceCode(t, err, "conflict")
}
func TestShardResourceTombstonesSingletonsAndPermissions(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	h.insert(t, "a", `{"title":"delete me"}`)
	command := service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: "a"}, Op: "delete", BaseRevision: "1", RequestID: uuid.NewString()}
	result, err := h.svc.Mutate(h.ctx, h.target, command)
	if err != nil || result.Tombstone.Revision != "2" {
		t.Fatalf("delete: %+v %v", result, err)
	}
	result, err = h.svc.Mutate(h.ctx, h.target, command)
	if err != nil || result.Tombstone.Revision != "2" {
		t.Fatalf("delete replay: %+v %v", result, err)
	}
	if len(h.read(t, service.ResourceRequest{Binding: "tasks"}).Records) != 0 {
		t.Fatal("tombstone returned live")
	}
	var retained string
	if err := h.pool.QueryRow(h.ctx, `SELECT data->>'title' FROM shard_resource_records WHERE shard_id=$1 AND id='a' AND deleted_at IS NOT NULL`, h.target.ShardID).Scan(&retained); err != nil || retained != "delete me" {
		t.Fatalf("tombstone lost recoverable data: %q %v", retained, err)
	}

	command.Op = "update"
	command.RequestID = uuid.NewString()
	command.Data = json.RawMessage(`{"title":"revive"}`)
	command.BaseRevision = "2"
	_, err = h.svc.Mutate(h.ctx, h.target, command)
	requireResourceCode(t, err, "not-found")
	singleton := service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "preferences"}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(`{"theme":"dark"}`)}
	result, err = h.svc.Mutate(h.ctx, h.target, singleton)
	if err != nil || result.Record.ID != "value" {
		t.Fatalf("singleton: %+v %v", result, err)
	}
	agent := h.target
	agent.Audience = "agent"
	singleton.Op = "update"
	singleton.ID = "value"
	singleton.BaseRevision = "1"
	singleton.RequestID = uuid.NewString()
	_, err = h.svc.Mutate(h.ctx, agent, singleton)
	requireResourceCode(t, err, "forbidden")
	readonly := service.WithPrincipal(context.Background(), service.Principal{UserID: testAdminUserID, ActorType: service.ActorTypeIntegrationToken, ActorID: "reader", Scopes: []string{service.ScopeArtifactsRead}})
	_, err = h.svc.Mutate(readonly, h.target, singleton)
	requireResourceCode(t, err, "forbidden")
}
func TestShardResourceQueriesAndCursorScope(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	h.insert(t, "a", `{"title":"a","score":2,"done":false}`)
	h.insert(t, "b", `{"title":"b","score":10,"done":true}`)
	h.insert(t, "c", `{"title":"c","score":null}`)
	h.insert(t, "d", `{"title":"d"}`)
	query := shardv2.Query{Limit: 2, OrderBy: []shardv2.Order{{Field: "/score", Direction: "desc"}}}
	first := h.read(t, service.ResourceRequest{Binding: "tasks", Query: &query})
	if len(first.Records) != 2 || first.Records[0].ID != "b" || first.Records[1].ID != "a" || first.NextCursor == "" {
		t.Fatalf("numeric sort: %+v", first)
	}
	query.Cursor = &first.NextCursor
	second := h.read(t, service.ResourceRequest{Binding: "tasks", Query: &query})
	if len(second.Records) != 2 || second.Records[0].ID != "c" || second.Records[1].ID != "d" {
		t.Fatalf("null/missing order: %+v", second)
	}
	query.OrderBy[0].Direction = "asc"
	_, err := h.svc.Read(h.ctx, h.target, service.ResourceRequest{Binding: "tasks", Query: &query})
	requireResourceCode(t, err, "stale-cursor")
	for _, tc := range []struct {
		predicate shardv2.Predicate
		want      string
	}{{shardv2.Predicate{Field: "/score", Op: "eq", Value: nil}, "c"}, {shardv2.Predicate{Field: "/score", Op: "exists", Value: false}, "d"}, {shardv2.Predicate{Field: "/score", Op: "gte", Value: 10.0}, "b"}} {
		page := h.read(t, service.ResourceRequest{Binding: "tasks", Query: &shardv2.Query{Where: &tc.predicate}})
		if len(page.Records) != 1 || page.Records[0].ID != tc.want {
			t.Fatalf("query %+v: %+v", tc.predicate, page)
		}
	}
	// Caller filters can narrow the binding's authored scope, never replace it.
	page := h.read(t, service.ResourceRequest{Binding: "openTasks", Query: &shardv2.Query{Where: &shardv2.Predicate{Field: "/done", Op: "eq", Value: true}}})
	if len(page.Records) != 0 {
		t.Fatal("caller replaced authored query")
	}
	_, err = h.svc.Read(h.ctx, h.target, service.ResourceRequest{Binding: "publicTasks", Query: &shardv2.Query{Where: &shardv2.Predicate{Field: "/score", Op: "eq", Value: 2.0}}})
	requireResourceCode(t, err, "forbidden")
	injection := shardv2.Predicate{Field: "/title", Op: "eq", Value: "' OR true --"}
	if len(h.read(t, service.ResourceRequest{Binding: "tasks", Query: &shardv2.Query{Where: &injection}}).Records) != 0 {
		t.Fatal("query value was executable")
	}
}
func TestShardResourceQuotaAdmissionIsAtomic(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{ActiveBytes: 20})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := h.svc.Mutate(h.ctx, h.target, service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: id}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(`{"title":"data"}`)})
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			requireResourceCode(t, err, "quota")
		}
	}
	if successes != 1 {
		t.Fatalf("quota allowed %d writes", successes)
	}
}
func TestShardResourceReleasePinningAndMigrationGate(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	h.insert(t, "a", `{"title":"saved"}`)
	contract := h.compiled.Contract
	definition := contract.Resources["tasks"]
	definition.Exposure.App = []string{"snapshot", "query", "observe"}
	contract.Resources["tasks"] = definition
	raw, _ := json.Marshal(contract)
	next, err := shardv2.Compile(raw, h.profiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.StageResourceRelease(h.ctx, h.target.ShardID, service.ChannelPublished, "build-2", "generation-1", next); err != nil {
		t.Fatal(err)
	}
	// Merely staging/draft authoring cannot change active grants.
	h.insert(t, "before-activation", `{"title":"still granted"}`)
	if err := h.repo.ActivateResourceRelease(h.ctx, h.target.ShardID, service.ChannelPublished, "build-2", h.profiles); err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.Mutate(h.ctx, h.target, service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks"}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(`{"title":"stale"}`)})
	requireResourceCode(t, err, "contract-changed")
	h.target.ContractHash = next.Hash
	_, err = h.svc.Mutate(h.ctx, h.target, service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks"}, Op: "insert", RequestID: uuid.NewString(), Data: json.RawMessage(`{"title":"denied"}`)})
	requireResourceCode(t, err, "forbidden")
	definition.SchemaVersion = 2
	contract.Resources["tasks"] = definition
	raw, _ = json.Marshal(contract)
	incompatible, err := shardv2.Compile(raw, h.profiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.StageResourceRelease(h.ctx, h.target.ShardID, service.ChannelPublished, "build-3", "generation-1", incompatible); err != nil {
		t.Fatal(err)
	}
	err = h.repo.ActivateResourceRelease(h.ctx, h.target.ShardID, service.ChannelPublished, "build-3", h.profiles)
	requireResourceCode(t, err, "conflict")
	active, err := h.repo.ActiveResourceRelease(h.ctx, testAdminUserID, h.target.ShardID, service.ChannelPublished)
	if err != nil || active.Hash != next.Hash {
		t.Fatal("failed activation changed the pointer")
	}
}
func TestShardResourceStreamReconcilesAndRevokes(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	sub, err := h.svc.Subscribe(h.ctx, h.target, service.ResourceRequest{Binding: "tasks"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	receive := func() service.ResourceStreamMessage {
		t.Helper()
		select {
		case msg := <-sub.Events:
			return msg
		case <-time.After(3 * time.Second):
			t.Fatal("stream timed out")
			return service.ResourceStreamMessage{}
		}
	}
	first := receive()
	if first.Event == nil || first.Event.Seq != "0" || len(first.Event.Records) != 0 {
		t.Fatalf("initial snapshot: %+v", first)
	}
	h.insert(t, "a", `{"title":"live"}`)
	next := receive()
	if next.Event == nil || next.Event.Seq != "1" || len(next.Event.Records) != 1 {
		t.Fatalf("refresh: %+v", next)
	}
	// No outbox drainer is running in this test: periodic reads still discover it.
	_, err = h.pool.Exec(h.ctx, `DELETE FROM artifacts WHERE id=$1`, h.target.ShardID)
	if err != nil {
		t.Fatal(err)
	}
	denied := receive()
	if denied.Error == nil || denied.Error.Code != "not-found" {
		t.Fatalf("revocation: %+v", denied)
	}
}

func TestShardResourceReceiptFailureRollsBackRecord(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	// A request-specific receipt constraint simulates failure after the primary
	// write but before its durable acknowledgement, without touching other tests.
	constraint := "test_receipt_" + uuid.NewString()[:8]
	_, err := h.pool.Exec(h.ctx, "ALTER TABLE shard_resource_receipts ADD CONSTRAINT "+constraint+" CHECK (request_id <> 'fault-"+h.target.ShardID+"')")
	if err != nil {
		t.Fatal(err)
	}
	defer h.pool.Exec(context.Background(), "ALTER TABLE shard_resource_receipts DROP CONSTRAINT "+constraint)
	command := service.ResourceMutation{ResourceRequest: service.ResourceRequest{Binding: "tasks", ID: "must-rollback"}, Op: "insert", RequestID: "fault-" + h.target.ShardID, Data: json.RawMessage(`{"title":"never committed"}`)}
	if _, err := h.svc.Mutate(h.ctx, h.target, command); err == nil {
		t.Fatal("receipt fault did not fail")
	}
	if len(h.read(t, service.ResourceRequest{Binding: "tasks"}).Records) != 0 {
		t.Fatal("record committed without receipt")
	}
	_, err = h.pool.Exec(h.ctx, "ALTER TABLE shard_resource_receipts DROP CONSTRAINT "+constraint)
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.svc.Mutate(h.ctx, h.target, command)
	if err != nil || result.Record.Revision != "1" {
		t.Fatalf("retry after rollback: %+v %v", result, err)
	}
}

func TestShardResourceSlowConsumerRetiresEpoch(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	sub, err := h.svc.Subscribe(h.ctx, h.target, service.ResourceRequest{Binding: "tasks"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	// Leave seq=0 unread. The next snapshot must produce an error, never silently
	// overwrite it with seq=1 or allocate an unbounded queue.
	h.insert(t, "a", `{"title":"overflow"}`)
	// Wait for producer closure, checking the error without relying on a sleep.
	// This is a timing test of a 10ms polling source. A bounded grace period
	// keeps the queue full while allowing the producer to discover the write.
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	select {
	case message := <-sub.Events:
		if message.Error == nil || message.Error.Code != "resync-required" {
			t.Fatalf("slow consumer got %+v", message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer was not retired")
	}
}
