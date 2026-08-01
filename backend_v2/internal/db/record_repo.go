package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRecordRepo struct{ pool *pgxpool.Pool }

func NewRecordRepository(pool *pgxpool.Pool) RecordRepository {
	return &pgRecordRepo{pool}
}

func (r *pgRecordRepo) UpsertCanonical(ctx context.Context, record *Record) (*RecordUpsertResult, error) {
	meta, err := json.Marshal(record.ProviderMetadata)
	if err != nil {
		return nil, fmt.Errorf("Record UpsertCanonical marshal metadata: %w", err)
	}
	var owner any
	if record.OwnerUserID != nil {
		owner = *record.OwnerUserID
	}

	row := r.pool.QueryRow(ctx, `
		WITH incoming AS (
			SELECT
				$1::text AS id,
				$2::text AS provider,
				$3::uuid AS provider_stream_id,
				$4::uuid AS owner_user_id,
				$5::text AS external_id,
				$6::bigint AS source_revision,
				$7::text AS type,
				$8::text AS label,
				$9::text AS content,
				$10::text AS source_url,
				$11::jsonb AS metadata
		),
		upserted AS (
			INSERT INTO records (
				id, provider, provider_stream_id, owner_user_id, external_id, source_revision,
				type, label, content, source_url, metadata, status, created_at
			)
			SELECT
				id, provider, provider_stream_id, owner_user_id, external_id, source_revision,
				type, label, content, source_url, metadata, 'captured', now()
			FROM incoming
			ON CONFLICT (id) DO UPDATE
			SET provider = EXCLUDED.provider,
			    provider_stream_id = EXCLUDED.provider_stream_id,
			    owner_user_id = EXCLUDED.owner_user_id,
			    external_id = EXCLUDED.external_id,
			    source_revision = EXCLUDED.source_revision,
			    type = EXCLUDED.type,
			    label = EXCLUDED.label,
			    content = EXCLUDED.content,
			    source_url = EXCLUDED.source_url,
			    metadata = EXCLUDED.metadata,
			    status = CASE WHEN records.status = 'pending' THEN 'captured' ELSE records.status END,
			    updated_at = now()
			WHERE records.source_revision < EXCLUDED.source_revision
			RETURNING records.*, (xmax = 0) AS inserted
		)
		SELECT
			id, provider_stream_id::text, provider, external_id, owner_user_id::text,
			type, label, COALESCE(source_url, ''), content, source_revision,
			metadata, COALESCE(enrichment, '{}'::jsonb), created_at, status,
			inserted, true AS changed, false AS stale
		FROM upserted
		UNION ALL
		SELECT
			existing.id, existing.provider_stream_id::text, existing.provider, existing.external_id,
			existing.owner_user_id::text, existing.type, existing.label, COALESCE(existing.source_url, ''),
			existing.content, existing.source_revision, existing.metadata,
			COALESCE(existing.enrichment, '{}'::jsonb), existing.created_at, existing.status,
			false AS inserted, false AS changed,
			(existing.source_revision >= incoming.source_revision) AS stale
		FROM records existing, incoming
		WHERE existing.id = incoming.id
		  AND NOT EXISTS (SELECT 1 FROM upserted)
		LIMIT 1
	`, record.ID, record.Provider, record.ProviderStreamID, owner, record.ExternalID, record.SourceRevision,
		record.Type, record.Title, record.ContentExcerpt, record.SourceURL, string(meta))

	out, err := scanRecordUpsert(row)
	if err != nil {
		return nil, fmt.Errorf("Record UpsertCanonical: %w", err)
	}
	return out, nil
}

func (r *pgRecordRepo) Get(ctx context.Context, id string) (*Record, error) {
	var meta, enrichment []byte
	record := &Record{}
	var owner *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(provider_stream_id::text, ''), COALESCE(provider, ''),
		       COALESCE(external_id, ''), owner_user_id::text,
		       type, label, COALESCE(source_url, ''), content, source_revision,
		       metadata, COALESCE(enrichment, '{}'::jsonb), created_at, status
		  FROM records
		 WHERE id = $1
	`, id).Scan(&record.ID, &record.ProviderStreamID, &record.Provider, &record.ExternalID, &owner,
		&record.Type, &record.Title, &record.SourceURL, &record.ContentExcerpt, &record.SourceRevision,
		&meta, &enrichment, &record.CreatedAt, &record.Status)
	if err != nil {
		return nil, fmt.Errorf("Record Get: %w", err)
	}
	record.OwnerUserID = owner
	_ = json.Unmarshal(meta, &record.ProviderMetadata)
	record.Enrichment = parseRecordEnrichment(record.ID, record.SourceRevision, enrichment)
	return record, nil
}

func (r *pgRecordRepo) SaveEnrichment(ctx context.Context, e *RecordEnrichment) (bool, error) {
	enrichment, err := marshalRecordEnrichment(e)
	if err != nil {
		return false, err
	}
	var embedding any
	if len(e.Embedding) > 0 {
		embedding = vectorToString(e.Embedding)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE records
		SET enrichment = $3::jsonb,
		    embedding = COALESCE($4::vector, embedding),
		    status = 'enriched',
		    updated_at = now()
		WHERE id = $1
		  AND source_revision = $2
	`, e.RecordID, e.SourceRevision, string(enrichment), embedding)
	if err != nil {
		return false, fmt.Errorf("Record SaveEnrichment: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SaveEmbedding writes a record's vector embedding and marks it `complete` — the terminal
// status of the record→knowledge chain (after enrichment and entity resolution). The
// source_revision guard drops the write if a newer revision has superseded the record.
func (r *pgRecordRepo) SaveEmbedding(ctx context.Context, recordID string, sourceRevision int64, vec []float32) (bool, error) {
	if len(vec) == 0 {
		return false, fmt.Errorf("Record SaveEmbedding: empty vector")
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE records
		   SET embedding = $3::vector,
		       status = 'complete',
		       updated_at = now()
		 WHERE id = $1
		   AND source_revision = $2
	`, recordID, sourceRevision, vectorToString(vec))
	if err != nil {
		return false, fmt.Errorf("Record SaveEmbedding: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ListStuck returns non-terminal records (not 'complete'/'failed') untouched for at least
// olderThanSecs — candidates the reaper re-drives. Oldest first.
func (r *pgRecordRepo) ListStuck(ctx context.Context, olderThanSecs, limit int) ([]*Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(provider_stream_id::text, ''), COALESCE(provider, ''),
		       COALESCE(external_id, ''), owner_user_id::text,
		       type, label, COALESCE(source_url, ''), content, source_revision,
		       metadata, COALESCE(enrichment, '{}'::jsonb), created_at, status
		  FROM records
		 WHERE status NOT IN ('complete', 'failed')
		   AND updated_at < now() - ($1 * interval '1 second')
		 ORDER BY updated_at ASC
		 LIMIT $2
	`, olderThanSecs, limit)
	if err != nil {
		return nil, fmt.Errorf("Record ListStuck: %w", err)
	}
	defer rows.Close()

	var out []*Record
	for rows.Next() {
		var meta, enrichment []byte
		var owner *string
		record := &Record{}
		if err := rows.Scan(&record.ID, &record.ProviderStreamID, &record.Provider, &record.ExternalID, &owner,
			&record.Type, &record.Title, &record.SourceURL, &record.ContentExcerpt, &record.SourceRevision,
			&meta, &enrichment, &record.CreatedAt, &record.Status); err != nil {
			return nil, fmt.Errorf("Record ListStuck scan: %w", err)
		}
		record.OwnerUserID = owner
		_ = json.Unmarshal(meta, &record.ProviderMetadata)
		record.Enrichment = parseRecordEnrichment(record.ID, record.SourceRevision, enrichment)
		out = append(out, record)
	}
	return out, rows.Err()
}

// MarkFailed moves a record to the terminal 'failed' status (after a permanent error), so
// the failure is visible and re-drivable rather than silently dropped. No-op if terminal.
func (r *pgRecordRepo) MarkFailed(ctx context.Context, recordID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE records SET status = 'failed', updated_at = now()
		 WHERE id = $1 AND status NOT IN ('complete', 'failed')
	`, recordID)
	if err != nil {
		return fmt.Errorf("Record MarkFailed: %w", err)
	}
	return nil
}

// ResetForRetry moves a 'failed' record back to 'captured' so the reaper re-drives it.
func (r *pgRecordRepo) ResetForRetry(ctx context.Context, recordID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE records SET status = 'captured', updated_at = now()
		 WHERE id = $1 AND status = 'failed'
	`, recordID)
	if err != nil {
		return false, fmt.Errorf("Record ResetForRetry: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SaveComplete writes a fully-processed tenant record to PG.
// Tenant/source matching context lives in tenant_item_matches, not records.
func (r *pgRecordRepo) SaveComplete(ctx context.Context, a *CompletedRecord) error {
	meta, _ := json.Marshal(a.Metadata)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO records
			(id, provider, provider_stream_id, owner_user_id, external_id, type, label, content, source_url,
			 metadata, enrichment, embedding, status, source_revision, created_at)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb, $12::vector, 'in_graph', $13, NOW())
		ON CONFLICT (id) DO UPDATE
		SET provider = EXCLUDED.provider,
		    provider_stream_id = EXCLUDED.provider_stream_id,
		    owner_user_id = EXCLUDED.owner_user_id,
		    type = EXCLUDED.type,
		    label = EXCLUDED.label,
		    content = EXCLUDED.content,
		    source_url = EXCLUDED.source_url,
		    metadata = EXCLUDED.metadata,
		    enrichment = EXCLUDED.enrichment,
		    embedding = EXCLUDED.embedding,
		    status = EXCLUDED.status,
		    source_revision = EXCLUDED.source_revision
		WHERE records.source_revision < EXCLUDED.source_revision
	`,
		a.ID, a.Provider, emptyIfNil(a.ProviderStreamID), emptyPtr(a.OwnerUserID), a.ExternalID,
		a.Type, a.Label, a.Content, a.SourceURL,
		string(meta), string(a.Enrichment), vectorToString(a.Embedding), a.SourceRevision,
	)
	if err != nil {
		return fmt.Errorf("SaveComplete: %w", err)
	}
	return nil
}

func scanRecordUpsert(row pgx.Row) (*RecordUpsertResult, error) {
	record := &Record{}
	var meta, enrichment []byte
	var owner *string
	var inserted, changed, stale bool
	err := row.Scan(&record.ID, &record.ProviderStreamID, &record.Provider, &record.ExternalID, &owner,
		&record.Type, &record.Title, &record.SourceURL, &record.ContentExcerpt, &record.SourceRevision,
		&meta, &enrichment, &record.CreatedAt, &record.Status, &inserted, &changed, &stale)
	if err != nil {
		return nil, err
	}
	record.OwnerUserID = owner
	_ = json.Unmarshal(meta, &record.ProviderMetadata)
	record.Enrichment = parseRecordEnrichment(record.ID, record.SourceRevision, enrichment)
	return &RecordUpsertResult{Record: record, Changed: changed, IsStale: stale, Inserted: inserted}, nil
}

func marshalRecordEnrichment(e *RecordEnrichment) ([]byte, error) {
	if e == nil {
		return []byte(`{}`), nil
	}
	return json.Marshal(struct {
		Summary               string   `json:"summary,omitempty"`
		Entities              []string `json:"entities,omitempty"`
		Topics                []string `json:"topics,omitempty"`
		KeyClaims             []string `json:"key_claims,omitempty"`
		LowConfidenceEntities []string `json:"low_confidence_entities,omitempty"`
		Status                string   `json:"status,omitempty"`
	}{
		Summary:               e.Summary,
		Entities:              e.Entities,
		Topics:                e.Topics,
		KeyClaims:             e.KeyClaims,
		LowConfidenceEntities: e.LowConfidenceEntities,
		Status:                e.Status,
	})
}

func parseRecordEnrichment(recordID string, sourceRevision int64, payload []byte) RecordEnrichment {
	e := RecordEnrichment{RecordID: recordID, SourceRevision: sourceRevision}
	var parsed struct {
		Summary               string   `json:"summary"`
		Entities              []string `json:"entities"`
		Topics                []string `json:"topics"`
		KeyClaims             []string `json:"key_claims"`
		LowConfidenceEntities []string `json:"low_confidence_entities"`
		Status                string   `json:"status"`
	}
	_ = json.Unmarshal(payload, &parsed)
	e.Summary = parsed.Summary
	e.Entities = parsed.Entities
	e.Topics = parsed.Topics
	e.KeyClaims = parsed.KeyClaims
	e.LowConfidenceEntities = parsed.LowConfidenceEntities
	e.Status = parsed.Status
	return e
}

func emptyIfNil(value string) string {
	return value
}

func emptyPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func vectorToString(v []float32) string {
	b, _ := json.Marshal(v)
	return string(b)
}
