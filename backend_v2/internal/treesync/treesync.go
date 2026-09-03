package treesync

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/outbox"
	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type scanner interface {
	Scan(dest ...any) error
}

// sync_source.go — projecting canonical state into sync frame entities.
//
// tree_nodes is the per-entity spine (every folder AND artifact has exactly one
// row; for artifacts tree_nodes.id == artifact_id). seq + is_deleted live there.
// The SAME light projection feeds two callers: the producers (one entity, in the
// write tx, post-mutation) and the cold-start snapshot (all entities, incl
// tombstones). Light fields only — heavy bodies (page blocks) are fetched on
// demand, never relayed.

// rowQuerier is satisfied by both pgx.Tx and *pgxpool.Pool.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// (scanner — Scan(dest ...any) error — is declared once in helpers.go and reused
// here; it is satisfied by both pgx.Row and pgx.Rows.)

// lightNodeData is the JSON `data` payload of an upsert frame entity — the light
// fields a tree/list view needs. Folder rows omit type/sourceUrl.
type lightNodeData struct {
	ID        string             `json:"id"`
	Kind      string             `json:"kind"`
	ParentID  *string            `json:"parentId"`
	Position  int64              `json:"position"`
	Title     *string            `json:"title"`
	Type      *string            `json:"type,omitempty"`
	SourceURL *string            `json:"sourceUrl,omitempty"`
	Summary   *string            `json:"summary,omitempty"`
	Metadata  json.RawMessage    `json:"metadata,omitempty"`
	Research  *lightResearchData `json:"research,omitempty"`
}

// lightResearchData is the research folder's extension row, projected to the LIGHT
// fields only — what a tree row or tab chip renders (§12's run dot, §8's batch mark).
// Set on research nodes, absent on every other kind, so folder/artifact payloads are
// byte-identical to before.
//
// Deliberately NOT carried: hypothesis, manifest, universe, code_hash. Those are the
// heavy bodies this file's doctrine says are fetched on demand, never relayed — the
// Overview reads them when you open it, and keeping them off the frame is what stops a
// manifest edit from re-broadcasting to every tree row.
type lightResearchData struct {
	RunState   string `json:"runState"`
	ExecMode   string `json:"execMode"`
	SourceKind string `json:"sourceKind"`
}

// lightEntitySelect projects a tree_nodes row (joined to its artifact, or to its
// research extension) to the light columns. $1 = user_id. Callers append a row filter
// (` AND n.id = $2`) or an ordering. It deliberately does NOT filter is_deleted —
// tombstones must flow to the snapshot; tombstone-hiding belongs to the read queries.
//
// Research nodes carry the folder shape (their own title, no artifact), so the title
// CASE treats them like folders.
const lightEntitySelect = `
	SELECT n.id, n.kind, n.parent_id, n.position,
	       CASE WHEN n.kind IN ('folder', 'research') THEN n.title ELSE a.title END AS title,
	       a.type, a.source_url, a.summary, a.metadata,
	       rs.run_state, rs.exec_mode, rs.source_kind,
	       n.seq, n.is_deleted
	  FROM tree_nodes n
	  LEFT JOIN artifacts a ON a.id = n.artifact_id AND a.user_id = n.user_id
	  LEFT JOIN research_strategies rs ON rs.node_id = n.id AND rs.user_id = n.user_id
	 WHERE n.user_id = $1::uuid`

// LightEntitySelect is the shared canonical projection used by domain adapters
// that must return the same light tree-node representation as sync frames.
const LightEntitySelect = lightEntitySelect

// scanLightEntity maps one projected row to a FrameEntity. A tombstoned row
// becomes an Op:delete (carrying its seq, no data); a live row an Op:upsert with
// its light data.
func scanLightEntity(row scanner) (service.FrameEntity, error) {
	var (
		id, kind   string
		parentID   *string
		position   int64
		title      *string
		aType      *string
		sourceURL  *string
		summary    *string
		metadata   []byte
		runState   *string
		execMode   *string
		sourceKind *string
		seq        int64
		isDeleted  bool
	)
	if err := row.Scan(&id, &kind, &parentID, &position, &title, &aType, &sourceURL, &summary, &metadata,
		&runState, &execMode, &sourceKind, &seq, &isDeleted); err != nil {
		return service.FrameEntity{}, err
	}
	ent := service.FrameEntity{EntityKind: kind, EntityID: id, Seq: uint64(seq)}
	if isDeleted {
		ent.Op = service.OpDelete
		return ent, nil
	}
	ent.Op = service.OpUpsert
	// The extension row is 1:1 and created with the node, so a research node always has
	// one. Guard anyway: a research node whose extension is somehow missing should still
	// sync as a tree row rather than fail the whole frame.
	var research *lightResearchData
	if kind == "research" && runState != nil {
		research = &lightResearchData{RunState: *runState, ExecMode: derefOr(execMode, "event"), SourceKind: derefOr(sourceKind, "authored")}
	}
	data, err := json.Marshal(lightNodeData{
		ID: id, Kind: kind, ParentID: parentID, Position: position,
		Title: title, Type: aType, SourceURL: sourceURL,
		Summary: summary, Metadata: json.RawMessage(metadata),
		Research: research,
	})
	if err != nil {
		return service.FrameEntity{}, err
	}
	ent.Data = data
	return ent, nil
}

// derefOr reads a nullable projected column, falling back to the column's schema
// default when the row is absent (the sparse-extension guard above).
func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

// lightEntityByID reads one node's current light projection (used by producers
// after a canonical mutation + seq bump, to build the frame entity).
func lightEntityByID(ctx context.Context, q rowQuerier, userID, nodeID string) (service.FrameEntity, error) {
	row := q.QueryRow(ctx, lightEntitySelect+` AND n.id = $2`, userID, nodeID)
	ent, err := scanLightEntity(row)
	if err != nil {
		return service.FrameEntity{}, fmt.Errorf("sync: light entity %s: %w", nodeID, err)
	}
	return ent, nil
}

// bumpNodeSeq increments the per-entity version on a node (the staleness guard).
// Call inside the write tx after mutating the entity, before reading it back.
func bumpNodeSeq(ctx context.Context, tx pgx.Tx, userID, nodeID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE tree_nodes SET seq = seq + 1, updated_at = now() WHERE id = $1 AND user_id = $2::uuid`,
		nodeID, userID); err != nil {
		return fmt.Errorf("sync: bump seq %s: %w", nodeID, err)
	}
	return nil
}

// emitNodeUpsert is the common producer path for a create or update of one tree
// node: bump its seq, read its current light projection, and append a
// single-entity upsert frame to the outbox — all in the caller's write tx, so
// the data and the event commit together. nodeID is the tree_nodes id (== the
// artifact id for artifacts; the entity is keyed by the node id either way).
func EmitNodeUpsert(ctx context.Context, tx pgx.Tx, userID, nodeID string) error {
	if err := bumpNodeSeq(ctx, tx, userID, nodeID); err != nil {
		return err
	}
	ent, err := lightEntityByID(ctx, tx, userID, nodeID)
	if err != nil {
		return err
	}
	return outbox.AppendData(ctx, tx, userID, service.Frame{Entities: []service.FrameEntity{ent}})
}

// softDeleteNode tombstones one node (is_deleted=true, seq bumped, row KEPT so a
// stale lower-seq upsert can't resurrect it) and returns its delete frame entity
// (carrying the bumped seq). Caller appends the frame (one frame may tombstone a
// whole subtree). entityKind is the node's kind ("folder" | "artifact").
func SoftDeleteNode(ctx context.Context, tx pgx.Tx, userID, nodeID, entityKind string) (service.FrameEntity, error) {
	var seq int64
	err := tx.QueryRow(ctx, `
		UPDATE tree_nodes
		   SET is_deleted = true, seq = seq + 1, updated_at = now()
		 WHERE id = $1 AND user_id = $2::uuid
		RETURNING seq
	`, nodeID, userID).Scan(&seq)
	if err != nil {
		return service.FrameEntity{}, fmt.Errorf("sync: soft delete %s: %w", nodeID, err)
	}
	return service.FrameEntity{
		EntityKind: entityKind,
		EntityID:   nodeID,
		Seq:        uint64(seq),
		Op:         service.OpDelete,
	}, nil
}

// TreeSyncSource is the cold-start snapshot provider for the workspace tree
// (folders + artifacts). Implements service.SyncSource.
type TreeSyncSource struct {
	pool *pgxpool.Pool
}

func NewTreeSyncSource(pool *pgxpool.Pool) *TreeSyncSource {
	return &TreeSyncSource{pool: pool}
}

func (s *TreeSyncSource) EntityKind() string { return "tree" }

// Snapshot returns every tree entity for userID INCLUDING tombstones, so the
// client's seq guard works on the snapshot and resurrection stays blocked on a
// fresh client.
func (s *TreeSyncSource) Snapshot(ctx context.Context, userID string) ([]service.FrameEntity, error) {
	rows, err := s.pool.Query(ctx, lightEntitySelect+` ORDER BY n.position ASC, n.created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("sync: snapshot query: %w", err)
	}
	defer rows.Close()

	out := make([]service.FrameEntity, 0)
	for rows.Next() {
		ent, err := scanLightEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("sync: snapshot scan: %w", err)
		}
		out = append(out, ent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sync: snapshot rows: %w", err)
	}
	return out, nil
}
