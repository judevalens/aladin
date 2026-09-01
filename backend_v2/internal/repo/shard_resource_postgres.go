package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQL is the first v2 adapter. It reuses the existing KV storage engine
// and transaction conventions, but never shares rows or write APIs with v1.
type ShardResourcePostgres struct {
	pool   *pgxpool.Pool
	limits ShardResourceLimits
}
type ShardResourceLimits struct {
	ActiveBytes  int64
	Records      int
	Receipts     int
	ReceiptBytes int64
	Cursors      int
	Builds       int
	BuildBytes   int64
}

func NewShardResourcePostgres(pool *pgxpool.Pool, limits ShardResourceLimits) *ShardResourcePostgres {
	if limits.ActiveBytes <= 0 {
		limits.ActiveBytes = 16 << 20
	}
	if limits.Records <= 0 {
		limits.Records = 10000
	}
	if limits.Receipts <= 0 {
		limits.Receipts = 10000
	}
	if limits.ReceiptBytes <= 0 {
		limits.ReceiptBytes = 32 << 20
	}
	if limits.Cursors <= 0 {
		limits.Cursors = 1024
	}
	if limits.Builds <= 0 {
		limits.Builds = 128
	}
	if limits.BuildBytes <= 0 {
		limits.BuildBytes = 128 << 20
	}
	return &ShardResourcePostgres{pool: pool, limits: limits}
}
func (*ShardResourcePostgres) Profile() shardv2.ProviderProfile {
	return shardv2.ProviderProfile{Version: 1, Owned: true, Operations: []string{"snapshot", "query", "insert", "update", "delete"}, Observation: "refresh-snapshots", ParamsSchema: shardv2.Schema{"type": "object", "additionalProperties": false}}
}
func (*ShardResourcePostgres) Authorize(ctx context.Context, view service.ResourceView) error {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if principal.UserID != view.Namespace.UserID || view.Namespace.DatasetID == "" || len(view.Params) != 0 {
		return service.ErrForbidden
	}
	return shardv2.ValidateQuery(view.Definition, view.Query)
}
func pgResourceHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func lockResourceNamespace(ctx context.Context, tx pgx.Tx, ns service.ResourceNamespace) error {
	// Distinct from the global sync outbox lock. Every activation and v2 write
	// takes this first; scope is one owner's shard/environment, not all users.
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 57291))`, ns.UserID+"/"+ns.ShardID+"/"+string(ns.Environment))
	if err != nil {
		return err
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id FROM artifacts WHERE id=$1 AND user_id=$2::uuid AND type='app' FOR SHARE`, ns.ShardID, ns.UserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	return err
}
func checkResourceRelease(ctx context.Context, tx pgx.Tx, ns service.ResourceNamespace) error {
	var hash, generation string
	err := tx.QueryRow(ctx, `SELECT r.contract_hash,r.generation FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id) WHERE a.user_id=$1::uuid AND a.shard_id=$2 AND a.environment=$3 FOR SHARE OF a`, ns.UserID, ns.ShardID, string(ns.Environment)).Scan(&hash, &generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	if err != nil {
		return err
	}
	if hash != ns.ContractHash || generation != ns.Generation {
		return service.ResourceFailure("contract-changed", "Resource release changed")
	}
	return nil
}
func (r *ShardResourcePostgres) ActiveResourceRelease(ctx context.Context, userID, shardID string, environment service.BuildChannel) (service.ResourceRelease, error) {
	var release service.ResourceRelease
	err := r.pool.QueryRow(ctx, `SELECT r.contract_source,r.contract_hash,r.build_id,r.generation FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id) JOIN artifacts owner ON owner.id=a.shard_id AND owner.user_id=a.user_id AND owner.type='app' WHERE a.user_id=$1::uuid AND a.shard_id=$2 AND a.environment=$3`, userID, shardID, string(environment)).Scan(&release.Source, &release.Hash, &release.BuildID, &release.Generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return release, service.ErrNotFound
	}
	return release, err
}

// StageResourceRelease and ActivateResourceRelease are INTERNAL build-pipeline
// hooks, intentionally absent from HTTP/MCP. The production builder uses
// StageResourceBuild to bind code to the contract; contract-only staging is for
// provider conformance and cannot serve an application.
func (r *ShardResourcePostgres) StageResourceRelease(ctx context.Context, shardID string, environment service.BuildChannel, buildID, generation string, compiled *shardv2.Compiled) error {
	return r.stageResourceBuild(ctx, shardID, environment, buildID, generation, compiled, nil)
}
func (r *ShardResourcePostgres) StageResourceBuild(ctx context.Context, shardID string, environment service.BuildChannel, buildID, generation string, compiled *shardv2.Compiled, files map[string][]byte) error {
	return r.stageResourceBuild(ctx, shardID, environment, buildID, generation, compiled, files)
}
func (r *ShardResourcePostgres) ActiveResourceBuild(ctx context.Context, shardID string, environment service.BuildChannel) (service.ShardRelease, error) {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return service.ShardRelease{}, err
	}
	var release service.ShardRelease
	var files []byte
	err = r.pool.QueryRow(ctx, `SELECT r.contract_source,r.contract_hash,r.build_id,r.generation,b.files FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id) LEFT JOIN shard_resource_builds b USING(user_id,shard_id,environment,build_id) JOIN artifacts owner ON owner.id=a.shard_id AND owner.user_id=a.user_id AND owner.type='app' WHERE a.user_id=$1::uuid AND a.shard_id=$2 AND a.environment=$3`, p.UserID, shardID, string(environment)).Scan(&release.Source, &release.Hash, &release.BuildID, &release.Generation, &files)
	if errors.Is(err, pgx.ErrNoRows) {
		return release, service.ErrNotFound
	}
	if err != nil {
		return release, err
	}
	if len(files) == 0 {
		return release, service.ResourceFailure("source-unavailable", "Active release has no verified code")
	}
	err = json.Unmarshal(files, &release.Files)
	return release, err
}
func (r *ShardResourcePostgres) stageResourceBuild(ctx context.Context, shardID string, environment service.BuildChannel, buildID, generation string, compiled *shardv2.Compiled, files map[string][]byte) error {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return err
	}
	if compiled == nil || buildID == "" || generation == "" || len(buildID) > 256 || len(generation) > 256 {
		return service.ResourceFailure("bad-request", "Invalid staged release")
	}
	sourceHash := sha256.Sum256(compiled.Source)
	if hex.EncodeToString(sourceHash[:]) != compiled.Hash {
		return service.ResourceFailure("invalid-schema", "Compiled source hash does not match")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ns := service.ResourceNamespace{UserID: p.UserID, ShardID: shardID, Environment: environment}
	if err := lockResourceNamespace(ctx, tx, ns); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO shard_resource_releases(user_id,shard_id,environment,build_id,contract_hash,generation,contract_source) VALUES($1::uuid,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, p.UserID, shardID, string(environment), buildID, compiled.Hash, generation, []byte(compiled.Source))
	if err != nil {
		return err
	}
	var hash, storedGeneration string
	if err := tx.QueryRow(ctx, `SELECT contract_hash,generation FROM shard_resource_releases WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND build_id=$4`, p.UserID, shardID, string(environment), buildID).Scan(&hash, &storedGeneration); err != nil {
		return err
	}
	if hash != compiled.Hash || storedGeneration != generation {
		return service.ResourceFailure("conflict", "A staged build is immutable")
	}
	if files != nil {
		if buildID != service.ShardBuildIdentity(compiled.Source, files) {
			return service.ResourceFailure("conflict", "Build identity mismatch")
		}
		raw, err := json.Marshal(files)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO shard_resource_builds(user_id,shard_id,environment,build_id,files) VALUES($1::uuid,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, p.UserID, shardID, string(environment), buildID, raw); err != nil {
			return err
		}
	}
	// Stages and rollback candidates share a bounded namespace budget. Never
	// silently delete active or previously published builds to admit new code.
	var buildCount int
	var buildBytes int64
	if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(octet_length(r.contract_source::text)+COALESCE(octet_length(b.files::text),0)),0) FROM shard_resource_releases r LEFT JOIN shard_resource_builds b USING(user_id,shard_id,environment,build_id) WHERE r.user_id=$1::uuid AND r.shard_id=$2 AND r.environment=$3`, p.UserID, shardID, string(environment)).Scan(&buildCount, &buildBytes); err != nil {
		return err
	}
	if buildCount > r.limits.Builds || buildBytes > r.limits.BuildBytes {
		return service.ResourceFailure("quota", "Retained build budget exceeded; review inactive build retention")
	}
	return tx.Commit(ctx)
}
func (r *ShardResourcePostgres) ActivateResourceRelease(ctx context.Context, shardID string, environment service.BuildChannel, buildID string, profiles shardv2.Registry) error {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ns := service.ResourceNamespace{UserID: p.UserID, ShardID: shardID, Environment: environment}
	if err := lockResourceNamespace(ctx, tx, ns); err != nil {
		return err
	}
	var source []byte
	var generation string
	if err := tx.QueryRow(ctx, `SELECT contract_source,generation FROM shard_resource_releases WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND build_id=$4`, p.UserID, shardID, string(environment), buildID).Scan(&source, &generation); err != nil {
		return err
	}
	compiled, err := shardv2.Compile(source, profiles)
	if err != nil {
		return err
	}
	var previousSource []byte
	var previousGeneration string
	err = tx.QueryRow(ctx, `SELECT r.contract_source,r.generation FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id) WHERE a.user_id=$1::uuid AND a.shard_id=$2 AND a.environment=$3`, p.UserID, shardID, string(environment)).Scan(&previousSource, &previousGeneration)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if len(previousSource) > 0 {
		var previous shardv2.Contract
		if err := json.Unmarshal(previousSource, &previous); err != nil {
			return err
		}
		// Conservative activation: no implicit data/schema migration. A separate
		// freeze/export/transform flow must handle incompatible shape changes.
		if previousGeneration != generation {
			return service.ResourceFailure("conflict", "Storage generation changes require migration")
		}
		for _, next := range compiled.Contract.Resources {
			for _, old := range previous.Resources {
				if old.Source.Dataset != "" && old.Source.Dataset == next.Source.Dataset && (old.SchemaVersion != next.SchemaVersion || pgResourceHash(old.Schema) != pgResourceHash(next.Schema) || old.Kind != next.Kind) {
					return service.ResourceFailure("conflict", "Schema changes require an explicit migration")
				}
			}
		}
	}
	// Verify every active stored record for each declared owned dataset, including
	// schema versions. This is bounded by the namespace's record/byte quotas.
	for _, definition := range compiled.Contract.Resources {
		if !profiles[definition.Source.Provider].Owned {
			continue
		}
		rows, err := tx.Query(ctx, `SELECT id,schema_version,data FROM shard_resource_records WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND generation=$4 AND dataset_id=$5 AND deleted_at IS NULL`, p.UserID, shardID, string(environment), generation, definition.Source.Dataset)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			var version int64
			var data []byte
			if err := rows.Scan(&id, &version, &data); err != nil {
				rows.Close()
				return err
			}
			value, err := shardv2.DecodeJSON(data)
			if err != nil || version != definition.SchemaVersion || (definition.Kind == "singleton" && id != "value") || shardv2.ValidateData(definition.Schema, value) != nil {
				rows.Close()
				return service.ResourceFailure("invalid-schema", "Stored data is incompatible with the staged release")
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO shard_resource_active(user_id,shard_id,environment,build_id) VALUES($1::uuid,$2,$3,$4) ON CONFLICT(user_id,shard_id,environment) DO UPDATE SET build_id=EXCLUDED.build_id,updated_at=now()`, p.UserID, shardID, string(environment), buildID)
	if err != nil {
		return err
	}
	if environment == service.ChannelPublished {
		// A successful build is only staged. Notify viewers in the activation
		// transaction, so they switch code AND data only after publication commits.
		payload, err := json.Marshal(map[string]any{"page_id": shardID, "protocol": "bridge/2", "buildId": buildID, "contractHash": compiled.Hash})
		if err != nil {
			return err
		}
		if err := appendAppEvent(ctx, tx, p.UserID, service.OutboxAppEvent{ResourceKind: "artifact", ResourceID: shardID, Operation: "published", Payload: payload}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type resourceReceipt struct {
	Result  service.ResourceMutationResult `json:"result"`
	Failure *service.ResourceError         `json:"error,omitempty"`
}

func (r *ShardResourcePostgres) Mutate(ctx context.Context, view service.ResourceView, command shardv2.Command) (service.ResourceMutationResult, error) {
	empty := service.ResourceMutationResult{}
	if err := r.Authorize(ctx, view); err != nil {
		return empty, err
	}
	raw, _ := json.Marshal(command)
	v, err := shardv2.DecodeJSON(raw)
	if err != nil || shardv2.ValidateProtocol("command", v) != nil {
		return empty, service.ResourceFailure("bad-request", "Invalid storage command")
	}
	if command.ContractHash != view.Namespace.ContractHash {
		return empty, service.ResourceFailure("contract-changed", "Command contract mismatch")
	}
	if command.Op != "delete" {
		value, err := shardv2.DecodeJSON(command.Data)
		if err != nil || len(command.Data) > shardv2.MaxRecordBytes || shardv2.ValidateData(view.Definition.Schema, value) != nil {
			return empty, service.ResourceFailure("invalid-schema", "Invalid stored record")
		}
		command.Data, _ = json.Marshal(value)
	}
	if view.Definition.Kind == "singleton" {
		if command.Op == "insert" && command.ID == "" {
			command.ID = "value"
		}
		if command.ID != "value" {
			return empty, service.ResourceFailure("bad-request", "Singleton ID must be value")
		}
	}
	ns := view.Namespace
	payloadHash := pgResourceHash([]any{ns, command})
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	if err := LockUser(ctx, tx, ns.UserID); err != nil {
		return empty, err
	}
	if err := lockResourceNamespace(ctx, tx, ns); err != nil {
		return empty, err
	}
	if err := checkResourceRelease(ctx, tx, ns); err != nil {
		return empty, err
	}
	var storedHash string
	var stored []byte
	err = tx.QueryRow(ctx, `SELECT payload_hash,outcome FROM shard_resource_receipts WHERE user_id=$1::uuid AND actor_key=$2 AND shard_id=$3 AND environment=$4 AND request_id=$5 AND expires_at>now()`, ns.UserID, ns.ActorKey, ns.ShardID, string(ns.Environment), command.RequestID).Scan(&storedHash, &stored)
	if err == nil {
		if storedHash != payloadHash {
			return empty, service.ResourceFailure("conflict", "requestId was used for a different command")
		}
		var receipt resourceReceipt
		if err := json.Unmarshal(stored, &receipt); err != nil {
			return empty, err
		}
		if receipt.Failure != nil {
			return receipt.Result, receipt.Failure
		}
		return receipt.Result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	// Prune only expired command receipts within this authorized namespace.
	if _, err := tx.Exec(ctx, `DELETE FROM shard_resource_receipts WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND expires_at<=now()`, ns.UserID, ns.ShardID, string(ns.Environment)); err != nil {
		return empty, err
	}
	var receiptCount int
	var receiptBytes int64
	if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(outcome_bytes),0) FROM shard_resource_receipts WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3`, ns.UserID, ns.ShardID, string(ns.Environment)).Scan(&receiptCount, &receiptBytes); err != nil {
		return empty, err
	}
	// Reserve the largest possible response before touching canonical data.
	if receiptCount >= r.limits.Receipts || receiptBytes+shardv2.MaxRecordBytes+2048 > r.limits.ReceiptBytes {
		return empty, service.ResourceFailure("quota", "Command receipt quota exceeded")
	}
	if command.ID == "" {
		command.ID = uuid.NewString()
	}
	result, failure, err := r.applyResourceCommand(ctx, tx, view, command)
	if err != nil {
		return empty, err
	}
	receipt := resourceReceipt{Result: result, Failure: failure}
	encoded, _ := json.Marshal(receipt)
	if _, err := tx.Exec(ctx, `INSERT INTO shard_resource_receipts(user_id,actor_key,shard_id,environment,request_id,payload_hash,outcome,outcome_bytes,expires_at) VALUES($1::uuid,$2,$3,$4,$5,$6,$7::jsonb,$8,now()+interval '24 hours')`, ns.UserID, ns.ActorKey, ns.ShardID, string(ns.Environment), command.RequestID, payloadHash, encoded, len(encoded)); err != nil {
		return empty, err
	}
	// Durable audit/invalidation in the same DB transaction. It contains no record
	// payload or credentials. Snapshot polling remains the recovery authority.
	if failure == nil {
		afterRevision := ""
		if result.Record != nil {
			afterRevision = result.Record.Revision
		}
		if result.Tombstone != nil {
			afterRevision = result.Tombstone.Revision
		}
		payload, _ := json.Marshal(map[string]any{"shardId": ns.ShardID, "environment": ns.Environment, "generation": ns.Generation, "resource": command.Resource, "id": command.ID, "requestId": command.RequestID, "actor": ns.ActorKey, "operation": command.Op, "beforeRevision": command.BaseRevision, "afterRevision": afterRevision, "result": "committed", "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
		if err := appendAppEvent(ctx, tx, ns.UserID, service.OutboxAppEvent{ResourceKind: "shard", ResourceID: ns.ShardID, Operation: "resource-changed", Payload: payload}); err != nil {
			return empty, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return empty, err
	}
	if failure != nil {
		return result, failure
	}
	return result, nil
}
func (r *ShardResourcePostgres) applyResourceCommand(ctx context.Context, tx pgx.Tx, view service.ResourceView, command shardv2.Command) (service.ResourceMutationResult, *service.ResourceError, error) {
	result := service.ResourceMutationResult{RequestID: command.RequestID}
	ns := view.Namespace
	fail := func(code, message string) (service.ResourceMutationResult, *service.ResourceError, error) {
		return result, &service.ResourceError{Code: code, Message: message}, nil
	}
	var revision, oldBytes int64
	var deleted *time.Time
	err := tx.QueryRow(ctx, `SELECT revision,data_bytes,deleted_at FROM shard_resource_records WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND generation=$4 AND dataset_id=$5 AND id=$6`, ns.UserID, ns.ShardID, string(ns.Environment), ns.Generation, ns.DatasetID, command.ID).Scan(&revision, &oldBytes, &deleted)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, nil, err
	}
	if command.Op == "insert" && exists {
		return fail("conflict", "Record ID already exists")
	}
	if command.Op != "insert" {
		if !exists || deleted != nil {
			return fail("not-found", "Record does not exist")
		}
		if strconv.FormatInt(revision, 10) != command.BaseRevision {
			return fail("conflict", "Record revision changed")
		}
	}
	if command.Op == "delete" {
		_, err := tx.Exec(ctx, `UPDATE shard_resource_records SET revision=revision+1,deleted_at=now(),updated_at=now(),updated_by=$7 WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND generation=$4 AND dataset_id=$5 AND id=$6`, ns.UserID, ns.ShardID, string(ns.Environment), ns.Generation, ns.DatasetID, command.ID, ns.ActorKey)
		result.Tombstone = &service.ResourceTombstone{ID: command.ID, Revision: strconv.FormatInt(revision+1, 10)}
		return result, nil, err
	}
	var activeBytes int64
	var count int
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(data_bytes) FILTER(WHERE deleted_at IS NULL),0),count(*) FROM shard_resource_records WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3`, ns.UserID, ns.ShardID, string(ns.Environment)).Scan(&activeBytes, &count)
	if err != nil {
		return result, nil, err
	}
	if activeBytes-oldBytes+int64(len(command.Data)) > r.limits.ActiveBytes || (!exists && count >= r.limits.Records) {
		return fail("quota", "Resource storage quota exceeded")
	}
	if command.Op == "insert" {
		_, err = tx.Exec(ctx, `INSERT INTO shard_resource_records(user_id,shard_id,environment,generation,dataset_id,id,schema_version,revision,data,data_bytes,created_by,updated_by) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,1,$8::jsonb,$9,$10,$10)`, ns.UserID, ns.ShardID, string(ns.Environment), ns.Generation, ns.DatasetID, command.ID, view.Definition.SchemaVersion, []byte(command.Data), len(command.Data), ns.ActorKey)
	} else {
		_, err = tx.Exec(ctx, `UPDATE shard_resource_records SET revision=revision+1,schema_version=$7,data=$8::jsonb,data_bytes=$9,updated_at=now(),updated_by=$10 WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND generation=$4 AND dataset_id=$5 AND id=$6`, ns.UserID, ns.ShardID, string(ns.Environment), ns.Generation, ns.DatasetID, command.ID, view.Definition.SchemaVersion, []byte(command.Data), len(command.Data), ns.ActorKey)
	}
	result.Record = &shardv2.Record{ID: command.ID, Revision: strconv.FormatInt(revision+1, 10), SchemaVersion: view.Definition.SchemaVersion, Data: command.Data}
	return result, nil, err
}

var _ service.ResourceProvider = (*ShardResourcePostgres)(nil)
var _ service.ResourceReleaseReader = (*ShardResourcePostgres)(nil)
