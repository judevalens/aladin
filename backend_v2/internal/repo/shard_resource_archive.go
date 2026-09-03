package repo

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/shardv2"
)

type resourceArchiveHeader struct {
	Format       string                    `json:"format"`
	UserID       string                    `json:"userId"`
	ShardID      string                    `json:"shardId"`
	Environment  shardresource.Environment `json:"environment"`
	Generation   string                    `json:"generation"`
	ContractHash string                    `json:"contractHash"`
}
type resourceArchiveRecord struct {
	Dataset   string         `json:"dataset"`
	Record    shardv2.Record `json:"record"`
	CreatedBy string         `json:"createdBy"`
	UpdatedBy string         `json:"updatedBy"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt *time.Time     `json:"deletedAt,omitempty"`
}
type resourceArchiveReceipt struct {
	Actor       string          `json:"actor"`
	RequestID   string          `json:"requestId"`
	PayloadHash string          `json:"payloadHash"`
	Outcome     json.RawMessage `json:"outcome"`
	CreatedAt   time.Time       `json:"createdAt"`
	ExpiresAt   time.Time       `json:"expiresAt"`
}

func (r *ShardResourcePostgres) ExportResourceData(ctx context.Context, id string, environment shardresource.Environment, writer io.Writer) (shardresource.ArchiveManifest, error) {
	var manifest shardresource.ArchiveManifest
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return manifest, err
	}
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return manifest, err
	}
	if p.ActorType == service.ActorTypeContentToken {
		return manifest, service.ErrForbidden
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manifest, err
	}
	defer tx.Rollback(ctx)
	ns := shardresource.Namespace{UserID: p.UserID, ShardID: id, Environment: environment}
	if err := lockResourceNamespace(ctx, tx, ns); err != nil {
		return manifest, err
	}
	var hash, generation string
	if err := tx.QueryRow(ctx, `SELECT r.contract_hash,r.generation FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id) WHERE a.user_id=$1::uuid AND a.shard_id=$2 AND a.environment=$3`, p.UserID, id, string(environment)).Scan(&hash, &generation); err != nil {
		return manifest, err
	}
	digest := sha256.New()
	emit := func(kind string, value any, hashed bool) error {
		raw, err := json.Marshal(map[string]any{"type": kind, "value": value})
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		if hashed {
			_, _ = digest.Write(raw)
		}
		n, err := writer.Write(raw)
		if err == nil && n != len(raw) {
			return io.ErrShortWrite
		}
		return err
	}
	if err := emit("header", resourceArchiveHeader{"shard-resource-archive/1", p.UserID, id, environment, generation, hash}, true); err != nil {
		return manifest, err
	}
	rows, err := tx.Query(ctx, `SELECT dataset_id,id,revision::text,schema_version,data,created_by,updated_by,created_at,updated_at,deleted_at FROM shard_resource_records WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 AND generation=$4 ORDER BY dataset_id,id`, p.UserID, id, string(environment), generation)
	if err != nil {
		return manifest, err
	}
	for rows.Next() {
		var row resourceArchiveRecord
		if err := rows.Scan(&row.Dataset, &row.Record.ID, &row.Record.Revision, &row.Record.SchemaVersion, &row.Record.Data, &row.CreatedBy, &row.UpdatedBy, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt); err != nil {
			rows.Close()
			return manifest, err
		}
		if err := emit("record", row, true); err != nil {
			rows.Close()
			return manifest, err
		}
		manifest.Records++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return manifest, err
	}
	rows, err = tx.Query(ctx, `SELECT actor_key,request_id,payload_hash,outcome,created_at,expires_at FROM shard_resource_receipts WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3 ORDER BY actor_key,request_id`, p.UserID, id, string(environment))
	if err != nil {
		return manifest, err
	}
	for rows.Next() {
		var row resourceArchiveReceipt
		if err := rows.Scan(&row.Actor, &row.RequestID, &row.PayloadHash, &row.Outcome, &row.CreatedAt, &row.ExpiresAt); err != nil {
			rows.Close()
			return manifest, err
		}
		if err := emit("receipt", row, true); err != nil {
			rows.Close()
			return manifest, err
		}
		manifest.Receipts++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return manifest, err
	}
	manifest.SHA256 = hex.EncodeToString(digest.Sum(nil))
	if err := emit("manifest", manifest, false); err != nil {
		return manifest, err
	}
	return manifest, tx.Commit(ctx)
}

func (r *ShardResourcePostgres) RestoreResourceData(ctx context.Context, id string, environment shardresource.Environment, reader io.Reader, profiles shardv2.Registry) (shardresource.ArchiveManifest, error) {
	var manifest shardresource.ArchiveManifest
	bad := func(message string) (shardresource.ArchiveManifest, error) {
		return manifest, shardresource.Failure("bad-request", message)
	}
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return manifest, err
	}
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return manifest, err
	}
	if p.ActorType == service.ActorTypeContentToken {
		return manifest, service.ErrForbidden
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return manifest, err
	}
	defer tx.Rollback(ctx)
	ns := shardresource.Namespace{UserID: p.UserID, ShardID: id, Environment: environment}
	if err := lockResourceNamespace(ctx, tx, ns); err != nil {
		return manifest, err
	}
	var source []byte
	if err := tx.QueryRow(ctx, `SELECT r.contract_source,r.contract_hash,r.generation FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id) WHERE a.user_id=$1::uuid AND a.shard_id=$2 AND a.environment=$3`, p.UserID, id, string(environment)).Scan(&source, &ns.ContractHash, &ns.Generation); err != nil {
		return manifest, err
	}
	compiled, err := shardv2.Compile(source, profiles)
	if err != nil {
		return manifest, err
	}
	datasets := map[string]shardv2.Resource{}
	for _, definition := range compiled.Contract.Resources {
		if profiles[definition.Source.Provider].Owned {
			datasets[definition.Source.Dataset] = definition
		}
	}
	var occupied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shard_resource_records WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3) OR EXISTS(SELECT 1 FROM shard_resource_receipts WHERE user_id=$1::uuid AND shard_id=$2 AND environment=$3)`, p.UserID, id, string(environment)).Scan(&occupied); err != nil {
		return manifest, err
	}
	if occupied {
		return manifest, shardresource.Failure("conflict", "Restore requires an empty namespace; never roll back over accepted writes")
	}
	// Streaming input: one bounded line in memory, with total storage caps below.
	limited := &io.LimitedReader{R: reader, N: (768 << 20) + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), shardv2.MaxJSONBytes)
	digest := sha256.New()
	headerSeen, footerSeen := false, false
	var activeBytes, receiptBytes int64
	for scanner.Scan() {
		raw := scanner.Bytes()
		if footerSeen {
			return bad("Trailing archive data")
		}
		var item struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		}
		if json.Unmarshal(raw, &item) != nil {
			return bad("Invalid archive JSON")
		}
		if !headerSeen && item.Type != "header" {
			return bad("Archive header required")
		}
		switch item.Type {
		case "header":
			if headerSeen {
				return bad("Duplicate archive header")
			}
			var header resourceArchiveHeader
			if json.Unmarshal(item.Value, &header) != nil || header.Format != "shard-resource-archive/1" || header.UserID != p.UserID || header.ShardID != id || header.Environment != environment || header.Generation != ns.Generation || header.ContractHash != ns.ContractHash {
				return bad("Archive namespace/release mismatch")
			}
			headerSeen = true
		case "record":
			var row resourceArchiveRecord
			if json.Unmarshal(item.Value, &row) != nil {
				return bad("Invalid archived record")
			}
			definition, ok := datasets[row.Dataset]
			if !ok {
				return bad("Archive contains an undeclared dataset")
			}
			value, err := shardv2.DecodeJSON(row.Record.Data)
			revision, revErr := strconv.ParseInt(row.Record.Revision, 10, 64)
			recordRaw, _ := json.Marshal(row.Record)
			recordValue, _ := shardv2.DecodeJSON(recordRaw)
			if err != nil || revErr != nil || revision < 1 || row.Record.SchemaVersion != definition.SchemaVersion || len(row.Record.Data) > shardv2.MaxRecordBytes || shardv2.ValidateProtocol("record", recordValue) != nil || shardv2.ValidateData(definition.Schema, value) != nil || (definition.Kind == "singleton" && row.Record.ID != "value") {
				return bad("Archived record violates the active schema")
			}
			if row.DeletedAt == nil {
				activeBytes += int64(len(row.Record.Data))
			}
			manifest.Records++
			if activeBytes > r.limits.ActiveBytes || manifest.Records > r.limits.Records {
				return manifest, shardresource.Failure("quota", "Archive exceeds record quota")
			}
			_, err = tx.Exec(ctx, `INSERT INTO shard_resource_records(user_id,shard_id,environment,generation,dataset_id,id,schema_version,revision,data,data_bytes,created_by,updated_by,created_at,updated_at,deleted_at) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, p.UserID, id, string(environment), ns.Generation, row.Dataset, row.Record.ID, row.Record.SchemaVersion, revision, row.Record.Data, len(row.Record.Data), row.CreatedBy, row.UpdatedBy, row.CreatedAt, row.UpdatedAt, row.DeletedAt)
			if err != nil {
				return manifest, err
			}
		case "receipt":
			var row resourceArchiveReceipt
			if json.Unmarshal(item.Value, &row) != nil || row.Actor == "" || row.RequestID == "" || len(row.RequestID) > 256 || row.PayloadHash == "" {
				return bad("Invalid archived receipt")
			}
			if _, err := shardv2.DecodeJSON(row.Outcome); err != nil {
				return bad("Invalid receipt outcome")
			}
			manifest.Receipts++
			receiptBytes += int64(len(row.Outcome))
			if manifest.Receipts > r.limits.Receipts || receiptBytes > r.limits.ReceiptBytes {
				return manifest, shardresource.Failure("quota", "Archive exceeds receipt quota")
			}
			_, err = tx.Exec(ctx, `INSERT INTO shard_resource_receipts(user_id,actor_key,shard_id,environment,request_id,payload_hash,outcome,outcome_bytes,created_at,expires_at) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, p.UserID, row.Actor, id, string(environment), row.RequestID, row.PayloadHash, row.Outcome, len(row.Outcome), row.CreatedAt, row.ExpiresAt)
			if err != nil {
				return manifest, err
			}
		case "manifest":
			var expected shardresource.ArchiveManifest
			manifest.SHA256 = hex.EncodeToString(digest.Sum(nil))
			if json.Unmarshal(item.Value, &expected) != nil || expected != manifest {
				return bad("Archive checksum/count mismatch")
			}
			footerSeen = true
		default:
			return bad(fmt.Sprintf("Unknown archive entry %q", item.Type))
		}
		if item.Type != "manifest" {
			_, _ = digest.Write(raw)
			_, _ = digest.Write([]byte{'\n'})
		}
	}
	if err := scanner.Err(); err != nil {
		return manifest, err
	}
	if limited.N == 0 {
		return bad("Archive exceeds total byte limit")
	}
	if !footerSeen {
		return bad("Incomplete archive")
	}
	return manifest, tx.Commit(ctx)
}
