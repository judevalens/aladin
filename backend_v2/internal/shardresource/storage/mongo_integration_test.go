package storage

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func mongoResourceHarness(t *testing.T) (*ShardResourceMongo, context.Context, service.ResourceView) {
	t.Helper()
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("TEST_MONGODB_URI is not configured")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	database := "aladin_shard_test_" + mongoIndexName("run", t.Name())
	t.Cleanup(func() { _ = client.Database(database).Drop(context.Background()) })
	repository := NewShardResourceMongo(client, database, ShardResourceLimits{ActiveBytes: 1 << 20, Records: 100, Receipts: 100, ReceiptBytes: 1 << 20, Cursors: 100})
	ctx := service.WithPrincipal(context.Background(), service.Principal{UserID: "owner", ActorID: "owner", ActorType: service.ActorTypeUserSession, Scopes: []string{service.ScopeArtifactsRead, service.ScopeArtifactsWrite}})
	definition := shardv2.Resource{URI: "shard://self/resources/tasks", Kind: "collection", SchemaVersion: 1, Schema: shardv2.Schema{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}, "rank": map[string]any{"type": []any{"number", "null"}}}, "required": []any{"title"}, "additionalProperties": false}, Source: shardv2.Source{Provider: "shard.documents", Version: 1, Dataset: "tasks"}, Operations: []string{"snapshot", "query", "insert", "update", "delete"}, Query: shardv2.QueryPolicy{FilterFields: []string{"/rank", "/title"}, SortFields: []string{"/rank", "/title"}, MaxLimit: 20}}
	view := service.ResourceView{Namespace: service.ResourceNamespace{UserID: "owner", ActorKey: "user:owner", ShardID: "shard", Environment: service.ChannelDraft, DatasetID: "tasks", Generation: "initial", ContractHash: "contract"}, Definition: definition, Params: map[string]any{}, Query: shardv2.Query{Limit: 2, OrderBy: []shardv2.Order{{Field: "/rank", Direction: "asc"}}}, ViewHash: "view"}
	return repository, ctx, view
}

func mongoCommand(op, id, requestID, revision string, data map[string]any) shardv2.Command {
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	return shardv2.Command{Op: op, Resource: "tasks", ID: id, RequestID: requestID, BaseRevision: revision, ContractHash: "contract", Data: raw}
}

func TestShardResourceMongoCRUDQueryCursorAndIsolation(t *testing.T) {
	repository, ctx, view := mongoResourceHarness(t)
	for _, input := range []struct {
		id   string
		rank any
	}{{"a", 2.0}, {"b", nil}, {"c", 1.0}} {
		result, err := repository.Mutate(ctx, view, mongoCommand("insert", input.id, "insert-"+input.id, "", map[string]any{"title": input.id, "rank": input.rank}))
		if err != nil || result.Record.Revision != "1" {
			t.Fatalf("insert %s: %+v %v", input.id, result, err)
		}
	}
	page, err := repository.Snapshot(ctx, view)
	if err != nil || len(page.Records) != 2 || page.Records[0].ID != "c" || page.Records[1].ID != "a" || page.NextCursor == "" {
		t.Fatalf("first page: %+v %v", page, err)
	}
	view.Query.Cursor = &page.NextCursor
	second, err := repository.Snapshot(ctx, view)
	if err != nil || len(second.Records) != 1 || second.Records[0].ID != "b" {
		t.Fatalf("second page: %+v %v", second, err)
	}
	other := view
	other.Namespace.ShardID = "other"
	other.Query.Cursor = nil
	other.ViewHash = "other"
	isolated, err := repository.Snapshot(ctx, other)
	if err != nil || len(isolated.Records) != 0 {
		t.Fatalf("namespace isolation: %+v %v", isolated, err)
	}
	filter := shardv2.Predicate{Field: "/rank", Op: "eq", Value: nil}
	view.Query = shardv2.Query{Limit: 10, Where: &filter}
	nulls, err := repository.Snapshot(ctx, view)
	if err != nil || len(nulls.Records) != 1 || nulls.Records[0].ID != "b" {
		t.Fatalf("null query: %+v %v", nulls, err)
	}
	typedIn := shardv2.Predicate{Field: "/title", Op: "in", Value: []string{"a", "c"}}
	view.Query = shardv2.Query{Limit: 10, Where: &typedIn}
	typed, err := repository.Snapshot(ctx, view)
	if err != nil || len(typed.Records) != 2 {
		t.Fatalf("typed in query: %+v %v", typed, err)
	}
}

func TestShardResourceMongoCASIdempotencyAndConcurrentReplay(t *testing.T) {
	repository, ctx, view := mongoResourceHarness(t)
	command := mongoCommand("insert", "a", "same", "", map[string]any{"title": "one", "rank": 1.0})
	const count = 8
	results := make(chan service.ResourceMutationResult, count)
	failures := make(chan error, count)
	var group sync.WaitGroup
	for range count {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := repository.Mutate(ctx, view, command)
			results <- result
			failures <- err
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatalf("concurrent replay: %v", err)
		}
	}
	for result := range results {
		if result.Record == nil || result.Record.ID != "a" || result.Record.Revision != "1" {
			t.Fatalf("receipt mismatch: %+v", result)
		}
	}
	_, err := repository.Mutate(ctx, view, mongoCommand("update", "a", "update", "9", map[string]any{"title": "bad"}))
	if service.ResourceErrorCode(err) != "conflict" {
		t.Fatalf("stale revision: %v", err)
	}
	updated, err := repository.Mutate(ctx, view, mongoCommand("update", "a", "update-2", "1", map[string]any{"title": "two"}))
	if err != nil || updated.Record.Revision != "2" {
		t.Fatalf("update: %+v %v", updated, err)
	}
	deleted, err := repository.Mutate(ctx, view, mongoCommand("delete", "a", "delete", "2", nil))
	if err != nil || deleted.Tombstone.Revision != "3" {
		t.Fatalf("delete: %+v %v", deleted, err)
	}
	_, receipts, _, _ := repository.collections(view.Namespace)
	if _, err := receipts.InsertOne(ctx, mongoReceipt{ActorKey: view.Namespace.ActorKey, RequestID: "expired", PayloadHash: "old", Outcome: []byte(`{"result":{"requestId":"expired"}}`), OutcomeBytes: 42, ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Mutate(ctx, view, mongoCommand("insert", "reused", "expired", "", map[string]any{"title": "reused"})); err != nil {
		t.Fatalf("expired request ID remained blocked before TTL sweep: %v", err)
	}
}

func TestShardResourceMongoOrderedChangeSignal(t *testing.T) {
	repository, ctx, view := mongoResourceHarness(t)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes, err := repository.ObserveChanges(watchCtx, view)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Mutate(ctx, view, mongoCommand("insert", "event", "event", "", map[string]any{"title": "event"})); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-changes:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("change stream did not signal")
	}
}

func TestShardResourceMongoFreezeExportAndRestore(t *testing.T) {
	repository, ctx, view := mongoResourceHarness(t)
	if _, err := repository.Mutate(ctx, view, mongoCommand("insert", "a", "a", "", map[string]any{"title": "portable", "rank": 1.0})); err != nil {
		t.Fatal(err)
	}
	if err := repository.FreezeNamespace(ctx, view.Namespace, true); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Mutate(ctx, view, mongoCommand("insert", "b", "b", "", map[string]any{"title": "blocked"})); service.ResourceErrorCode(err) != "conflict" {
		t.Fatalf("frozen write accepted: %v", err)
	}
	archive, err := repository.ExportNamespace(ctx, view.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	target := view
	target.Namespace.Generation = "migrated"
	target.ViewHash = "migrated"
	if err := repository.FreezeNamespace(ctx, view.Namespace, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.RestoreNamespace(ctx, target.Namespace, archive); service.ResourceErrorCode(err) != "conflict" {
		t.Fatalf("unfenced restore accepted: %v", err)
	}
	if err := repository.FreezeNamespace(ctx, view.Namespace, true); err != nil {
		t.Fatal(err)
	}
	if err := repository.RestoreNamespace(ctx, target.Namespace, archive); err != nil {
		t.Fatal(err)
	}
	page, err := repository.Snapshot(ctx, target)
	if err != nil || len(page.Records) != 1 || page.Records[0].ID != "a" {
		t.Fatalf("restored: %+v %v", page, err)
	}
	if err := repository.FreezeNamespace(ctx, view.Namespace, false); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Mutate(ctx, view, mongoCommand("insert", "b", "b2", "", map[string]any{"title": "allowed"})); err != nil {
		t.Fatalf("unfrozen write: %v", err)
	}
}
