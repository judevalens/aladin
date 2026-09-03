package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	shardrelease "aladin/backend_v2/internal/shardresource/release"
	"aladin/backend_v2/internal/shardv2"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

// ShardResourceMongo is the default owned-data adapter for Shard v2. Postgres
// remains the release/control-plane authority; this adapter receives only the
// trusted ResourceView produced by the service layer.
//
// Each user/shard/environment receives opaque physical collections. Dataset and
// generation remain server-owned fields inside those collections so migrations
// and recovery do not depend on collection names.
type ShardResourceMongo struct {
	client  *mongo.Client
	db      *mongo.Database
	limits  ShardResourceLimits
	indexed sync.Map
}
type mongoIndexState struct {
	done chan struct{}
	err  error
}

type mongoRecord struct {
	ID            string         `bson:"id" json:"id"`
	DatasetID     string         `bson:"datasetId" json:"datasetId"`
	Generation    string         `bson:"generation" json:"generation"`
	SchemaVersion int64          `bson:"schemaVersion" json:"schemaVersion"`
	Revision      int64          `bson:"revision" json:"revision"`
	Data          map[string]any `bson:"data" json:"data"`
	DataBytes     int64          `bson:"dataBytes" json:"dataBytes"`
	CreatedAt     time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time      `bson:"updatedAt" json:"updatedAt"`
	CreatedBy     string         `bson:"createdBy" json:"createdBy"`
	UpdatedBy     string         `bson:"updatedBy" json:"updatedBy"`
	DeletedAt     *time.Time     `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
}

type mongoReceipt struct {
	ActorKey     string    `bson:"actorKey"`
	RequestID    string    `bson:"requestId"`
	PayloadHash  string    `bson:"payloadHash"`
	Outcome      []byte    `bson:"outcome"`
	OutcomeBytes int64     `bson:"outcomeBytes"`
	ExpiresAt    time.Time `bson:"expiresAt"`
}

type mongoCursor struct {
	Token     string    `bson:"token"`
	ActorKey  string    `bson:"actorKey"`
	ViewHash  string    `bson:"viewHash"`
	Offset    int64     `bson:"offset"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type mongoResourceEvent struct {
	DatasetID      string    `bson:"datasetId" json:"datasetId"`
	Generation     string    `bson:"generation" json:"generation"`
	Resource       string    `bson:"resource" json:"resource"`
	RecordID       string    `bson:"recordId" json:"recordId"`
	Operation      string    `bson:"operation" json:"operation"`
	BeforeRevision string    `bson:"beforeRevision,omitempty" json:"beforeRevision,omitempty"`
	AfterRevision  string    `bson:"afterRevision" json:"afterRevision"`
	RequestID      string    `bson:"requestId" json:"requestId"`
	ActorKey       string    `bson:"actorKey" json:"actorKey"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
	ExpiresAt      time.Time `bson:"expiresAt" json:"expiresAt"`
}

func NewShardResourceMongo(client *mongo.Client, database string, limits ShardResourceLimits) *ShardResourceMongo {
	if database == "" {
		database = "aladin_shards"
	}
	return &ShardResourceMongo{client: client, db: client.Database(database), limits: normalizeShardResourceLimits(limits)}
}

func (*ShardResourceMongo) Profile() shardv2.ProviderProfile {
	return shardv2.ProviderProfile{
		Version: 1, Owned: true,
		Operations:   []string{"snapshot", "query", "insert", "update", "delete"},
		Observation:  "ordered-changes",
		ParamsSchema: shardv2.Schema{"type": "object", "additionalProperties": false},
	}
}

func (r *ShardResourceMongo) ObserveChanges(ctx context.Context, view shardresource.View) (<-chan error, error) {
	if err := r.Authorize(ctx, view); err != nil {
		return nil, err
	}
	if err := r.ensureIndexes(ctx, view); err != nil {
		return nil, err
	}
	_, _, _, events := r.collections(view.Namespace)
	pipeline := mongo.Pipeline{{{Key: "$match", Value: bson.M{
		"operationType":           "insert",
		"fullDocument.datasetId":  view.Namespace.DatasetID,
		"fullDocument.generation": view.Namespace.Generation,
	}}}}
	stream, err := events.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		return nil, err
	}
	changes := make(chan error, 1)
	go func() {
		defer close(changes)
		defer stream.Close(context.Background())
		for stream.Next(ctx) {
			select {
			case changes <- nil:
			case <-ctx.Done():
				return
			}
		}
		if err := stream.Err(); err != nil && ctx.Err() == nil {
			changes <- err
		}
	}()
	return changes, nil
}

func (r *ShardResourceMongo) ValidateResourceStage(_ context.Context, definition shardv2.Resource) error {
	if definition.Source.Provider != "shard.documents" {
		return nil
	}
	for _, pointer := range append(append([]string{}, definition.Query.FilterFields...), definition.Query.SortFields...) {
		if _, err := mongoDataPath(pointer); err != nil {
			return shardresource.Failure("unsupported-capability", err.Error())
		}
	}
	return nil
}

func (r *ShardResourceMongo) Authorize(ctx context.Context, view shardresource.View) error {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if principal.UserID != view.Namespace.UserID || view.Namespace.DatasetID == "" || len(view.Params) != 0 {
		return service.ErrForbidden
	}
	return shardv2.ValidateQuery(view.Definition, view.Query)
}

func mongoNamespaceHash(ns shardresource.Namespace) string {
	raw, _ := json.Marshal([]string{ns.UserID, ns.ShardID, string(ns.Environment)})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

func (r *ShardResourceMongo) collections(ns shardresource.Namespace) (records, receipts, cursors, events *mongo.Collection) {
	prefix := mongoNamespaceHash(ns)
	return r.db.Collection("records_" + prefix), r.db.Collection("receipts_" + prefix), r.db.Collection("cursors_" + prefix), r.db.Collection("events_" + prefix)
}
func (r *ShardResourceMongo) stateCollection(ns shardresource.Namespace) *mongo.Collection {
	return r.db.Collection("state_" + mongoNamespaceHash(ns))
}

func mongoDataPath(pointer string) (string, error) {
	parts, err := shardv2.PointerParts(pointer)
	if err != nil || len(parts) == 0 {
		return "", fmt.Errorf("invalid Mongo query field %q", pointer)
	}
	for _, part := range parts {
		if strings.Contains(part, ".") || strings.HasPrefix(part, "$") || part == "" {
			return "", fmt.Errorf("Mongo query field %q contains an unsupported key", pointer)
		}
	}
	return "data." + strings.Join(parts, "."), nil
}

func mongoIndexName(prefix, value string) string {
	sum := sha256.Sum256([]byte(value))
	return prefix + "_" + hex.EncodeToString(sum[:6])
}

func (r *ShardResourceMongo) ensureIndexes(ctx context.Context, view shardresource.View) error {
	key := mongoNamespaceHash(view.Namespace) + ":" + view.Namespace.ContractHash
	candidate := &mongoIndexState{done: make(chan struct{})}
	actual, loaded := r.indexed.LoadOrStore(key, candidate)
	if loaded {
		state := actual.(*mongoIndexState)
		select {
		case <-state.done:
			return state.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	candidate.err = r.createIndexes(ctx, view)
	close(candidate.done)
	if candidate.err != nil {
		r.indexed.Delete(key)
	}
	return candidate.err
}

func (r *ShardResourceMongo) createIndexes(ctx context.Context, view shardresource.View) error {
	records, receipts, cursors, events := r.collections(view.Namespace)
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "datasetId", Value: 1}, {Key: "generation", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetName("dataset_record").SetUnique(true)},
		{Keys: bson.D{{Key: "datasetId", Value: 1}, {Key: "generation", Value: 1}, {Key: "deletedAt", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetName("dataset_live_id")},
	}
	fields := append(append([]string{}, view.Definition.Query.FilterFields...), view.Definition.Query.SortFields...)
	seen := map[string]bool{}
	for _, pointer := range fields {
		path, err := mongoDataPath(pointer)
		if err != nil {
			return shardresource.Failure("unsupported-capability", err.Error())
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		models = append(models, mongo.IndexModel{Keys: bson.D{{Key: "datasetId", Value: 1}, {Key: "generation", Value: 1}, {Key: "deletedAt", Value: 1}, {Key: path, Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetName(mongoIndexName("field", path))})
	}
	if _, err := records.Indexes().CreateMany(ctx, models); err != nil {
		return err
	}
	if _, err := receipts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "actorKey", Value: 1}, {Key: "requestId", Value: 1}}, Options: options.Index().SetName("actor_request").SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetName("receipt_ttl").SetExpireAfterSeconds(0)},
	}); err != nil {
		return err
	}
	if _, err := cursors.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetName("cursor_token").SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetName("cursor_ttl").SetExpireAfterSeconds(0)},
		{Keys: bson.D{{Key: "actorKey", Value: 1}, {Key: "viewHash", Value: 1}, {Key: "offset", Value: 1}}, Options: options.Index().SetName("cursor_reuse")},
	}); err != nil {
		return err
	}
	if _, err := events.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "datasetId", Value: 1}, {Key: "generation", Value: 1}, {Key: "createdAt", Value: 1}}, Options: options.Index().SetName("dataset_events")},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetName("event_ttl").SetExpireAfterSeconds(0)},
	}); err != nil {
		return err
	}
	return nil
}

func mongoPredicate(p shardv2.Predicate) (bson.M, error) {
	if len(p.And) > 0 || len(p.Or) > 0 {
		children, operator := p.And, "$and"
		if len(p.Or) > 0 {
			children, operator = p.Or, "$or"
		}
		compiled := make(bson.A, 0, len(children))
		for _, child := range children {
			filter, err := mongoPredicate(child)
			if err != nil {
				return nil, err
			}
			compiled = append(compiled, filter)
		}
		return bson.M{operator: compiled}, nil
	}
	field, err := mongoDataPath(p.Field)
	if err != nil {
		return nil, err
	}
	if p.Op == "exists" {
		return bson.M{field: bson.M{"$exists": p.Value == true}}, nil
	}
	if p.Op == "eq" && p.Value == nil {
		return bson.M{field: bson.M{"$type": 10}}, nil
	}
	if p.Op == "in" {
		values, _ := p.Value.([]any)
		nonNull := bson.A{}
		hasNull := false
		for _, value := range values {
			if value == nil {
				hasNull = true
			} else {
				nonNull = append(nonNull, value)
			}
		}
		parts := bson.A{}
		if len(nonNull) > 0 {
			parts = append(parts, bson.M{field: bson.M{"$in": nonNull}})
		}
		if hasNull {
			parts = append(parts, bson.M{field: bson.M{"$type": 10}})
		}
		if len(parts) == 1 {
			return parts[0].(bson.M), nil
		}
		return bson.M{"$or": parts}, nil
	}
	operator := map[string]string{"eq": "$eq", "gt": "$gt", "gte": "$gte", "lt": "$lt", "lte": "$lte"}[p.Op]
	if operator == "" {
		return nil, fmt.Errorf("unsupported Mongo predicate %q", p.Op)
	}
	return bson.M{field: bson.M{operator: p.Value}}, nil
}

func mongoFilter(view shardresource.View) (bson.M, error) {
	filter := bson.M{"datasetId": view.Namespace.DatasetID, "generation": view.Namespace.Generation, "deletedAt": bson.M{"$exists": false}}
	if view.ID != "" {
		filter["id"] = view.ID
	}
	if view.Query.Where != nil {
		where, err := mongoPredicate(*view.Query.Where)
		if err != nil {
			return nil, err
		}
		filter["$and"] = bson.A{where}
	}
	return filter, nil
}

func mongoRecordEnvelope(record mongoRecord) (shardv2.Record, error) {
	raw, err := json.Marshal(record.Data)
	if err != nil {
		return shardv2.Record{}, err
	}
	return shardv2.Record{ID: record.ID, Revision: strconv.FormatInt(record.Revision, 10), SchemaVersion: record.SchemaVersion, Data: raw}, nil
}

func (r *ShardResourceMongo) Snapshot(ctx context.Context, view shardresource.View) (shardresource.Page, error) {
	empty := shardresource.Page{}
	if err := r.Authorize(ctx, view); err != nil {
		return empty, err
	}
	normalized, err := shardv2.NormalizeQuery(view.Query)
	if err != nil {
		return empty, shardresource.Failure("bad-request", "Invalid resource query")
	}
	view.Query = normalized
	if err := r.ensureIndexes(ctx, view); err != nil {
		return empty, err
	}
	records, _, cursors, _ := r.collections(view.Namespace)
	offset := int64(0)
	if view.Query.Cursor != nil && *view.Query.Cursor != "" {
		var cursor mongoCursor
		err := cursors.FindOne(ctx, bson.M{"token": *view.Query.Cursor, "actorKey": view.Namespace.ActorKey, "viewHash": view.ViewHash, "expiresAt": bson.M{"$gt": time.Now()}}).Decode(&cursor)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return empty, shardresource.Failure("stale-cursor", "Resource cursor expired or belongs to another view")
		}
		if err != nil {
			return empty, err
		}
		offset = cursor.Offset
	}
	filter, err := mongoFilter(view)
	if err != nil {
		return empty, shardresource.Failure("unsupported-capability", err.Error())
	}
	pipeline := mongo.Pipeline{{{Key: "$match", Value: filter}}}
	sortDoc := bson.D{}
	setDoc := bson.D{}
	for index, order := range view.Query.OrderBy {
		path, err := mongoDataPath(order.Field)
		if err != nil {
			return empty, shardresource.Failure("unsupported-capability", err.Error())
		}
		missingKey := fmt.Sprintf("__aladinSortMissing%d", index)
		fieldRef := "$" + path
		setDoc = append(setDoc, bson.E{Key: missingKey, Value: bson.M{"$cond": bson.A{bson.M{"$or": bson.A{bson.M{"$eq": bson.A{bson.M{"$type": fieldRef}, "missing"}}, bson.M{"$eq": bson.A{fieldRef, nil}}}}, 1, 0}}})
		sortDoc = append(sortDoc, bson.E{Key: missingKey, Value: 1})
		direction := 1
		if order.Direction == "desc" {
			direction = -1
		}
		sortDoc = append(sortDoc, bson.E{Key: path, Value: direction})
	}
	if len(setDoc) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$set", Value: setDoc}})
	}
	sortDoc = append(sortDoc, bson.E{Key: "id", Value: 1})
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: sortDoc}},
		bson.D{{Key: "$skip", Value: offset}},
		bson.D{{Key: "$limit", Value: int64(view.Query.Limit + 1)}},
	)
	stream, err := records.Aggregate(ctx, pipeline)
	if err != nil {
		return empty, err
	}
	defer stream.Close(ctx)
	page := shardresource.Page{Records: []shardv2.Record{}}
	for stream.Next(ctx) {
		var stored mongoRecord
		if err := stream.Decode(&stored); err != nil {
			return empty, err
		}
		record, err := mongoRecordEnvelope(stored)
		if err != nil {
			return empty, err
		}
		page.Records = append(page.Records, record)
	}
	if err := stream.Err(); err != nil {
		return empty, err
	}
	if len(page.Records) <= view.Query.Limit {
		return page, nil
	}
	page.Records = page.Records[:view.Query.Limit]
	nextOffset := offset + int64(len(page.Records))
	var existing mongoCursor
	err = cursors.FindOne(ctx, bson.M{"actorKey": view.Namespace.ActorKey, "viewHash": view.ViewHash, "offset": nextOffset, "expiresAt": bson.M{"$gt": time.Now()}}, options.FindOne().SetSort(bson.D{{Key: "expiresAt", Value: -1}})).Decode(&existing)
	if err == nil {
		page.NextCursor = existing.Token
		return page, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return empty, err
	}
	count, err := cursors.CountDocuments(ctx, bson.M{"expiresAt": bson.M{"$gt": time.Now()}})
	if err != nil {
		return empty, err
	}
	if count >= int64(r.limits.Cursors) {
		return empty, shardresource.Failure("quota", "Resource cursor quota exceeded")
	}
	page.NextCursor = uuid.NewString()
	_, err = cursors.InsertOne(ctx, mongoCursor{Token: page.NextCursor, ActorKey: view.Namespace.ActorKey, ViewHash: view.ViewHash, Offset: nextOffset, ExpiresAt: time.Now().Add(15 * time.Minute)})
	return page, err
}

func mongoPayloadHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func decodeMongoReceipt(stored mongoReceipt) (shardresource.MutationResult, error) {
	var receipt resourceReceipt
	if err := json.Unmarshal(stored.Outcome, &receipt); err != nil {
		return shardresource.MutationResult{}, err
	}
	if receipt.Failure != nil {
		return receipt.Result, receipt.Failure
	}
	return receipt.Result, nil
}

func (r *ShardResourceMongo) Mutate(ctx context.Context, view shardresource.View, command shardv2.Command) (shardresource.MutationResult, error) {
	empty := shardresource.MutationResult{}
	if err := r.Authorize(ctx, view); err != nil {
		return empty, err
	}
	if err := r.ensureIndexes(ctx, view); err != nil {
		return empty, err
	}
	raw, _ := json.Marshal(command)
	value, err := shardv2.DecodeJSON(raw)
	if err != nil || shardv2.ValidateProtocol("command", value) != nil {
		return empty, shardresource.Failure("bad-request", "Invalid storage command")
	}
	if command.ContractHash != view.Namespace.ContractHash {
		return empty, shardresource.Failure("contract-changed", "Command contract mismatch")
	}
	var document map[string]any
	if command.Op != "delete" {
		decoded, err := shardv2.DecodeJSON(command.Data)
		if err != nil || len(command.Data) > shardv2.MaxRecordBytes || shardv2.ValidateData(view.Definition.Schema, decoded) != nil {
			return empty, shardresource.Failure("invalid-schema", "Invalid stored record")
		}
		document = decoded.(map[string]any)
		command.Data, _ = json.Marshal(decoded)
	}
	if view.Definition.Kind == "singleton" {
		if command.Op == "insert" && command.ID == "" {
			command.ID = "value"
		}
		if command.ID != "value" {
			return empty, shardresource.Failure("bad-request", "Singleton ID must be value")
		}
	}
	if command.ID == "" {
		command.ID = uuid.NewString()
	}
	payloadHash := mongoPayloadHash([]any{view.Namespace, command})
	records, receipts, _, events := r.collections(view.Namespace)
	var finalReceipt resourceReceipt
	session, err := r.client.StartSession()
	if err != nil {
		return empty, err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
		state := r.stateCollection(view.Namespace)
		if _, err := state.UpdateOne(txCtx, bson.M{"_id": "namespace"}, bson.M{"$inc": bson.M{"fence": 1}}, options.UpdateOne().SetUpsert(true)); err != nil {
			return nil, err
		}
		var control struct {
			Frozen bool `bson:"frozen"`
		}
		if err := state.FindOne(txCtx, bson.M{"_id": "namespace"}).Decode(&control); err != nil {
			return nil, err
		}
		if control.Frozen {
			return nil, shardresource.Failure("conflict", "Resource namespace is frozen for migration")
		}
		var stored mongoReceipt
		err := receipts.FindOne(txCtx, bson.M{"actorKey": view.Namespace.ActorKey, "requestId": command.RequestID, "expiresAt": bson.M{"$gt": time.Now()}}).Decode(&stored)
		if err == nil {
			if stored.PayloadHash != payloadHash {
				return nil, shardresource.Failure("conflict", "requestId was used for a different command")
			}
			if err := json.Unmarshal(stored.Outcome, &finalReceipt); err != nil {
				return nil, err
			}
			return nil, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		// TTL deletion is asynchronous. Remove expired receipts transactionally so
		// a request ID becomes reusable as soon as its documented window ends.
		if _, err := receipts.DeleteMany(txCtx, bson.M{"expiresAt": bson.M{"$lte": time.Now()}}); err != nil {
			return nil, err
		}
		receiptCount, err := receipts.CountDocuments(txCtx, bson.M{"expiresAt": bson.M{"$gt": time.Now()}})
		if err != nil {
			return nil, err
		}
		if receiptCount >= int64(r.limits.Receipts) {
			return nil, shardresource.Failure("quota", "Command receipt quota exceeded")
		}
		receiptBytes := int64(0)
		usage, err := receipts.Aggregate(txCtx, mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"expiresAt": bson.M{"$gt": time.Now()}}}},
			{{Key: "$group", Value: bson.M{"_id": nil, "bytes": bson.M{"$sum": "$outcomeBytes"}}}},
		})
		if err != nil {
			return nil, err
		}
		if usage.Next(txCtx) {
			var total struct {
				Bytes int64 `bson:"bytes"`
			}
			if err := usage.Decode(&total); err != nil {
				usage.Close(txCtx)
				return nil, err
			}
			receiptBytes = total.Bytes
		}
		if err := usage.Err(); err != nil {
			usage.Close(txCtx)
			return nil, err
		}
		usage.Close(txCtx)
		result, failure, err := r.applyMongoCommand(txCtx, records, view, command, document)
		if err != nil {
			return nil, err
		}
		finalReceipt = resourceReceipt{Result: result, Failure: failure}
		encoded, _ := json.Marshal(finalReceipt)
		if receiptBytes+int64(len(encoded)) > r.limits.ReceiptBytes {
			return nil, shardresource.Failure("quota", "Command receipt quota exceeded")
		}
		_, err = receipts.InsertOne(txCtx, mongoReceipt{ActorKey: view.Namespace.ActorKey, RequestID: command.RequestID, PayloadHash: payloadHash, Outcome: encoded, OutcomeBytes: int64(len(encoded)), ExpiresAt: time.Now().Add(24 * time.Hour)})
		if err != nil {
			return nil, err
		}
		if failure == nil {
			afterRevision := ""
			if result.Record != nil {
				afterRevision = result.Record.Revision
			}
			if result.Tombstone != nil {
				afterRevision = result.Tombstone.Revision
			}
			now := time.Now().UTC()
			_, err = events.InsertOne(txCtx, mongoResourceEvent{DatasetID: view.Namespace.DatasetID, Generation: view.Namespace.Generation, Resource: command.Resource, RecordID: command.ID, Operation: command.Op, BeforeRevision: command.BaseRevision, AfterRevision: afterRevision, RequestID: command.RequestID, ActorKey: view.Namespace.ActorKey, CreatedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)})
		}
		return nil, err
	}, options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			var stored mongoReceipt
			if findErr := receipts.FindOne(ctx, bson.M{"actorKey": view.Namespace.ActorKey, "requestId": command.RequestID, "expiresAt": bson.M{"$gt": time.Now()}}).Decode(&stored); findErr == nil && stored.PayloadHash == payloadHash {
				return decodeMongoReceipt(stored)
			}
		}
		return empty, err
	}
	if finalReceipt.Failure != nil {
		return finalReceipt.Result, finalReceipt.Failure
	}
	return finalReceipt.Result, nil
}

func (r *ShardResourceMongo) applyMongoCommand(ctx context.Context, records *mongo.Collection, view shardresource.View, command shardv2.Command, data map[string]any) (shardresource.MutationResult, *shardresource.Error, error) {
	result := shardresource.MutationResult{RequestID: command.RequestID}
	fail := func(code, message string) (shardresource.MutationResult, *shardresource.Error, error) {
		return result, &shardresource.Error{Code: code, Message: message}, nil
	}
	base := bson.M{"datasetId": view.Namespace.DatasetID, "generation": view.Namespace.Generation, "id": command.ID}
	var existing mongoRecord
	err := records.FindOne(ctx, base).Decode(&existing)
	exists := err == nil
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return result, nil, err
	}
	if command.Op == "insert" && exists {
		return fail("conflict", "Record ID already exists")
	}
	if command.Op != "insert" {
		if !exists || existing.DeletedAt != nil {
			return fail("not-found", "Record does not exist")
		}
		if strconv.FormatInt(existing.Revision, 10) != command.BaseRevision {
			return fail("conflict", "Record revision changed")
		}
	}
	if command.Op == "delete" {
		now := time.Now().UTC()
		filter := bson.M{"datasetId": view.Namespace.DatasetID, "generation": view.Namespace.Generation, "id": command.ID, "revision": existing.Revision, "deletedAt": bson.M{"$exists": false}}
		update := bson.M{"$inc": bson.M{"revision": 1}, "$set": bson.M{"deletedAt": now, "updatedAt": now, "updatedBy": view.Namespace.ActorKey}}
		outcome, err := records.UpdateOne(ctx, filter, update)
		if err != nil {
			return result, nil, err
		}
		if outcome.ModifiedCount != 1 {
			return fail("conflict", "Record revision changed")
		}
		result.Tombstone = &shardresource.Tombstone{ID: command.ID, Revision: strconv.FormatInt(existing.Revision+1, 10)}
		return result, nil, nil
	}
	activePipeline := mongo.Pipeline{{{Key: "$match", Value: bson.M{"deletedAt": bson.M{"$exists": false}}}}, {{Key: "$group", Value: bson.M{"_id": nil, "bytes": bson.M{"$sum": "$dataBytes"}}}}}
	activeBytes := int64(0)
	stream, err := records.Aggregate(ctx, activePipeline)
	if err != nil {
		return result, nil, err
	}
	if stream.Next(ctx) {
		var totals struct {
			Bytes int64 `bson:"bytes"`
		}
		if err := stream.Decode(&totals); err != nil {
			stream.Close(ctx)
			return result, nil, err
		}
		activeBytes = totals.Bytes
	}
	stream.Close(ctx)
	recordCount, err := records.CountDocuments(ctx, bson.M{})
	if err != nil {
		return result, nil, err
	}
	oldBytes := existing.DataBytes
	if activeBytes-oldBytes+int64(len(command.Data)) > r.limits.ActiveBytes || (!exists && recordCount >= int64(r.limits.Records)) {
		return fail("quota", "Resource storage quota exceeded")
	}
	now := time.Now().UTC()
	if command.Op == "insert" {
		stored := mongoRecord{ID: command.ID, DatasetID: view.Namespace.DatasetID, Generation: view.Namespace.Generation, SchemaVersion: view.Definition.SchemaVersion, Revision: 1, Data: data, DataBytes: int64(len(command.Data)), CreatedAt: now, UpdatedAt: now, CreatedBy: view.Namespace.ActorKey, UpdatedBy: view.Namespace.ActorKey}
		if _, err := records.InsertOne(ctx, stored); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return fail("conflict", "Record ID already exists")
			}
			return result, nil, err
		}
	} else {
		filter := bson.M{"datasetId": view.Namespace.DatasetID, "generation": view.Namespace.Generation, "id": command.ID, "revision": existing.Revision, "deletedAt": bson.M{"$exists": false}}
		update := bson.M{"$inc": bson.M{"revision": 1}, "$set": bson.M{"schemaVersion": view.Definition.SchemaVersion, "data": data, "dataBytes": int64(len(command.Data)), "updatedAt": now, "updatedBy": view.Namespace.ActorKey}}
		outcome, err := records.UpdateOne(ctx, filter, update)
		if err != nil {
			return result, nil, err
		}
		if outcome.ModifiedCount != 1 {
			return fail("conflict", "Record revision changed")
		}
	}
	revision := existing.Revision + 1
	result.Record = &shardv2.Record{ID: command.ID, Revision: strconv.FormatInt(revision, 10), SchemaVersion: view.Definition.SchemaVersion, Data: command.Data}
	return result, nil, nil
}

var _ shardresource.Provider = (*ShardResourceMongo)(nil)
var _ shardrelease.StageValidator = (*ShardResourceMongo)(nil)
var _ shardresource.ChangeObserver = (*ShardResourceMongo)(nil)
var _ shardrelease.ActivationFence = (*ShardResourceMongo)(nil)

type mongoResourceArchive struct {
	Format     int                  `json:"format"`
	Generation string               `json:"generation"`
	Records    []mongoRecord        `json:"records"`
	Events     []mongoResourceEvent `json:"events"`
}

// FreezeNamespace is an internal migration fence. Mutations touch the same
// control document in their transaction, so a successful freeze drains older
// writers through MongoDB write-conflict retry and rejects newer writers.
func (r *ShardResourceMongo) FreezeNamespace(ctx context.Context, ns shardresource.Namespace, frozen bool) error {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil || principal.UserID != ns.UserID {
		return service.ErrForbidden
	}
	_, err = r.stateCollection(ns).UpdateOne(ctx, bson.M{"_id": "namespace"}, bson.M{"$set": bson.M{"frozen": frozen, "updatedAt": time.Now().UTC()}, "$inc": bson.M{"fence": 1}}, options.UpdateOne().SetUpsert(true))
	return err
}

// ExportNamespace returns a portable, versioned archive of durable records and
// audit events. Receipts and cursors are intentionally omitted because they are
// expiring transport state, not shard data.
func (r *ShardResourceMongo) ExportNamespace(ctx context.Context, ns shardresource.Namespace) ([]byte, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil || principal.UserID != ns.UserID {
		return nil, service.ErrForbidden
	}
	records, _, _, events := r.collections(ns)
	archive := mongoResourceArchive{Format: 1, Generation: ns.Generation, Records: []mongoRecord{}, Events: []mongoResourceEvent{}}
	session, err := r.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
		if err := r.requireFrozen(txCtx, ns); err != nil {
			return nil, err
		}
		recordCursor, err := records.Find(txCtx, bson.M{"generation": ns.Generation}, options.Find().SetSort(bson.D{{Key: "datasetId", Value: 1}, {Key: "id", Value: 1}}))
		if err != nil {
			return nil, err
		}
		if err := recordCursor.All(txCtx, &archive.Records); err != nil {
			return nil, err
		}
		eventCursor, err := events.Find(txCtx, bson.M{"generation": ns.Generation}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
		if err != nil {
			return nil, err
		}
		if err := eventCursor.All(txCtx, &archive.Events); err != nil {
			return nil, err
		}
		return nil, nil
	}, options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()))
	if err != nil {
		return nil, err
	}
	return json.Marshal(archive)
}

func (r *ShardResourceMongo) requireFrozen(ctx context.Context, ns shardresource.Namespace) error {
	state := r.stateCollection(ns)
	if _, err := state.UpdateOne(ctx, bson.M{"_id": "namespace"}, bson.M{"$inc": bson.M{"fence": 1}}); err != nil {
		return err
	}
	var control struct {
		Frozen bool `bson:"frozen"`
	}
	if err := state.FindOne(ctx, bson.M{"_id": "namespace"}).Decode(&control); err != nil || !control.Frozen {
		return shardresource.Failure("conflict", "Freeze namespace before archive operations")
	}
	return nil
}

// RestoreNamespace imports into an empty generation. Activation remains a
// separate Postgres control-plane action after schema validation.
func (r *ShardResourceMongo) RestoreNamespace(ctx context.Context, ns shardresource.Namespace, raw []byte) error {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil || principal.UserID != ns.UserID {
		return service.ErrForbidden
	}
	var archive mongoResourceArchive
	if err := json.Unmarshal(raw, &archive); err != nil || archive.Format != 1 {
		return shardresource.Failure("bad-request", "Unsupported shard archive")
	}
	records, _, _, events := r.collections(ns)
	if len(archive.Records) > r.limits.Records {
		return shardresource.Failure("quota", "Archive exceeds record quota")
	}
	var bytes int64
	for index := range archive.Records {
		record := &archive.Records[index]
		record.Generation = ns.Generation
		encoded, _ := json.Marshal(record.Data)
		record.DataBytes = int64(len(encoded))
		bytes += record.DataBytes
	}
	if bytes > r.limits.ActiveBytes {
		return shardresource.Failure("quota", "Archive exceeds storage quota")
	}
	for index := range archive.Events {
		archive.Events[index].Generation = ns.Generation
	}
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
		if err := r.requireFrozen(txCtx, ns); err != nil {
			return nil, err
		}
		count, err := records.CountDocuments(txCtx, bson.M{"generation": ns.Generation})
		if err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, shardresource.Failure("conflict", "Restore generation is not empty")
		}
		if len(archive.Records) > 0 {
			documents := make([]any, len(archive.Records))
			for i := range archive.Records {
				documents[i] = archive.Records[i]
			}
			if _, err := records.InsertMany(txCtx, documents); err != nil {
				return nil, err
			}
		}
		if len(archive.Events) > 0 {
			documents := make([]any, len(archive.Events))
			for i := range archive.Events {
				documents[i] = archive.Events[i]
			}
			if _, err := events.InsertMany(txCtx, documents); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}, options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()))
	return err
}
