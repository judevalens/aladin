package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Bridge is a shared entity worth a discourse pass — one mentioned by >= minDegree
// distinct documents. Sourced from the resolved entity layer (entity_mentions), keyed on
// entity_id (the rework of buck's Neo4j canonical-text bridges).
type Bridge struct {
	EntityID   string
	EntityName string
	Degree     int
}

// BridgeMember is a document that mentions a bridge entity (the discourse judge reads these).
type BridgeMember struct {
	ID        string
	Kind      string // record (artifact comes with the authored path, B6)
	OwnerID   string
	Label     string
	SourceURL string
	Summary   string
}

// EntityDiscourse is the per-entity sweep ledger state.
type EntityDiscourse struct {
	EntityID            string
	ConnectedCount      int
	CountAtLastAnalysis int
	LastAnalyzedAt      *time.Time
	Version             int
}

// DiscourseInsight is a stored discourse map for one entity (insights row, type=discourse).
type DiscourseInsight struct {
	EntityID   string
	EntityName string
	Title      string
	Body       string
	MemberIDs  []string
	Provenance []byte // JSON
	Confidence float64
	TrustTier  string
	Version    int
}

type pgDiscourseRepo struct{ pool *pgxpool.Pool }

func NewDiscourseRepository(pool *pgxpool.Pool) DiscourseRepository {
	return &pgDiscourseRepo{pool}
}

// CandidateBridges returns entities mentioned by >= minDegree distinct documents, best
// first — the entities worth analyzing. A bridge from the resolved layer, not Neo4j.
func (r *pgDiscourseRepo) CandidateBridges(ctx context.Context, minDegree, limit int) ([]Bridge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.entity_id::text, e.canonical_name, count(DISTINCT m.record_id) AS degree
		  FROM entity_mentions m JOIN entities e ON e.id = m.entity_id
		 GROUP BY m.entity_id, e.canonical_name
		HAVING count(DISTINCT m.record_id) >= $1
		 ORDER BY degree DESC
		 LIMIT $2
	`, minDegree, limit)
	if err != nil {
		return nil, fmt.Errorf("discourse CandidateBridges: %w", err)
	}
	defer rows.Close()
	var out []Bridge
	for rows.Next() {
		var b Bridge
		if err := rows.Scan(&b.EntityID, &b.EntityName, &b.Degree); err != nil {
			return nil, fmt.Errorf("discourse CandidateBridges scan: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BridgeMembers returns the documents that mention an entity (with label/summary for the judge).
func (r *pgDiscourseRepo) BridgeMembers(ctx context.Context, entityID string, limit int) ([]BridgeMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, COALESCE(r.owner_user_id::text, ''), r.label,
		       COALESCE(r.source_url, ''), COALESCE(r.enrichment->>'summary', '')
		  FROM entity_mentions m JOIN records r ON r.id = m.record_id
		 WHERE m.entity_id = $1::uuid
		 ORDER BY r.created_at DESC
		 LIMIT $2
	`, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("discourse BridgeMembers: %w", err)
	}
	defer rows.Close()
	var out []BridgeMember
	for rows.Next() {
		m := BridgeMember{Kind: "record"}
		if err := rows.Scan(&m.ID, &m.OwnerID, &m.Label, &m.SourceURL, &m.Summary); err != nil {
			return nil, fmt.Errorf("discourse BridgeMembers scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetDiscourseLedger returns the sweep state for an entity, or nil if never analyzed.
func (r *pgDiscourseRepo) GetDiscourseLedger(ctx context.Context, entityID string) (*EntityDiscourse, error) {
	var e EntityDiscourse
	err := r.pool.QueryRow(ctx, `
		SELECT entity_id::text, connected_count, count_at_last_analysis, last_analyzed_at, version
		  FROM entity_discourse WHERE entity_id = $1::uuid
	`, entityID).Scan(&e.EntityID, &e.ConnectedCount, &e.CountAtLastAnalysis, &e.LastAnalyzedAt, &e.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("discourse GetDiscourseLedger: %w", err)
	}
	return &e, nil
}

// MarkAnalyzed records that an entity was just analyzed at the given degree, bumping its
// version, and returns the new version.
func (r *pgDiscourseRepo) MarkAnalyzed(ctx context.Context, entityID string, degree int) (int, error) {
	var version int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO entity_discourse (entity_id, connected_count, count_at_last_analysis, last_analyzed_at, version, updated_at)
		VALUES ($1::uuid, $2, $2, now(), 1, now())
		ON CONFLICT (entity_id) DO UPDATE
		SET connected_count = EXCLUDED.connected_count,
		    count_at_last_analysis = EXCLUDED.count_at_last_analysis,
		    last_analyzed_at = now(),
		    version = entity_discourse.version + 1,
		    updated_at = now()
		RETURNING version
	`, entityID, degree).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("discourse MarkAnalyzed: %w", err)
	}
	return version, nil
}

// StoreDiscourse upserts the discourse map for an entity (one insights row, type=discourse).
func (r *pgDiscourseRepo) StoreDiscourse(ctx context.Context, d *DiscourseInsight) error {
	members, err := json.Marshal(d.MemberIDs)
	if err != nil {
		return fmt.Errorf("discourse StoreDiscourse marshal members: %w", err)
	}
	prov := d.Provenance
	if len(prov) == 0 {
		prov = []byte("{}")
	}
	trust := d.TrustTier
	if trust == "" {
		trust = "believed"
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO insights (kg_id, type, title, body, record_ids, entity, entity_id, confidence, user_status, provenance, trust_tier, version)
		VALUES (NULL, 'discourse', $1, $2, $3::jsonb, $4, $5::uuid, $6, 'pending', $7::jsonb, $8, $9)
		ON CONFLICT (entity_id) WHERE type = 'discourse'
		DO UPDATE SET title = EXCLUDED.title, body = EXCLUDED.body, record_ids = EXCLUDED.record_ids,
		    confidence = EXCLUDED.confidence, provenance = EXCLUDED.provenance, trust_tier = EXCLUDED.trust_tier,
		    version = EXCLUDED.version, created_at = now()
	`, d.Title, d.Body, string(members), d.EntityName, d.EntityID, d.Confidence, string(prov), trust, d.Version)
	if err != nil {
		return fmt.Errorf("discourse StoreDiscourse: %w", err)
	}
	return nil
}
