package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Data-layer redesign, Phase A — the workspace change feed (server side).
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// workspace_changes is a per-user append-only log of field-level changes. Its
// BIGSERIAL seq is both the client's pull cursor and the per-(entity,field)
// newest-wins comparator source. Writers append a change row in the SAME
// transaction as the entity write, under the per-user advisory lock; readers
// pull coalesced deltas since their cursor.

type ChangeOp string

const (
	OpCreate ChangeOp = "create"
	OpUpdate ChangeOp = "update"
	OpDelete ChangeOp = "delete"
)

// Change is one row of the workspace_changes feed. Field/Value are set only for
// OpUpdate (field-level); create/delete are entity-level (Field/Value nil).
type Change struct {
	Seq        int64           `json:"seq"`
	EntityKind string          `json:"entityKind"`
	EntityID   string          `json:"entityId"`
	Op         ChangeOp        `json:"op"`
	Field      *string         `json:"field,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	MutationID *string         `json:"mutationId,omitempty"`
}

type SyncRepo struct {
	pool *pgxpool.Pool
}

func NewSyncPostgres(pool *pgxpool.Pool) *SyncRepo {
	return &SyncRepo{pool: pool}
}

// LockUser serializes a user's writes so their workspace_changes.seq becomes
// visible in commit order. BIGSERIAL assigns seq at INSERT, but transactions
// can COMMIT out of assignment order — without this, a reader could observe
// seq=N+1 before seq=N commits and skip N forever (silent loss). Call at the
// start of every write transaction for that user, before the entity write +
// AppendChange. Released automatically at txn end (advisory_xact_lock).
func LockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, userID); err != nil {
		return fmt.Errorf("sync: lock user: %w", err)
	}
	return nil
}

// AppendChange inserts one field-level change row inside the caller's tx (which
// must already hold LockUser and the entity write). Returns the assigned seq.
// Rows from one logical command should share a MutationID (transaction group).
func AppendChange(ctx context.Context, tx pgx.Tx, userID string, c Change) (int64, error) {
	var seq int64
	err := tx.QueryRow(ctx, `
		INSERT INTO workspace_changes (user_id, entity_kind, entity_id, op, field, value, mutation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING seq`,
		userID, c.EntityKind, c.EntityID, string(c.Op), c.Field, c.Value, c.MutationID,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("sync: append change: %w", err)
	}
	return seq, nil
}

// PullResult is a delta response: coalesced changes (latest per (entity,field))
// in seq order, plus the new cursor (the feed high-water for this user, or the
// input cursor when nothing is newer).
type PullResult struct {
	Changes []Change `json:"changes"`
	Cursor  int64    `json:"cursor"`
}

// PullDelta returns the coalesced latest-per-(entity,field) changes with
// seq > cursor, in seq order. Coalescing collapses N edits of one field to its
// latest (e.g. 10 renames → 1). Create/delete (field NULL) coalesce per entity.
//
// A cold client (cursor == 0) instead gets a full SNAPSHOT of the current
// workspace built from the canonical tables (so pre-feed entities — created
// before the change feed existed — are included), stamped at the feed
// high-water. After that it pulls deltas from that cursor.
func (r *SyncRepo) PullDelta(ctx context.Context, userID string, cursor int64) (PullResult, error) {
	if cursor == 0 {
		return r.snapshot(ctx, userID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT seq, entity_kind, entity_id, op, field, value, mutation_id
		FROM (
			SELECT DISTINCT ON (entity_id, COALESCE(field, ''))
			       seq, entity_kind, entity_id, op, field, value, mutation_id
			FROM workspace_changes
			WHERE user_id = $1 AND seq > $2
			ORDER BY entity_id, COALESCE(field, ''), seq DESC
		) latest
		ORDER BY seq`,
		userID, cursor,
	)
	if err != nil {
		return PullResult{}, fmt.Errorf("sync: pull delta: %w", err)
	}
	defer rows.Close()

	out := PullResult{Changes: []Change{}, Cursor: cursor}
	for rows.Next() {
		var c Change
		var op string
		if err := rows.Scan(&c.Seq, &c.EntityKind, &c.EntityID, &op, &c.Field, &c.Value, &c.MutationID); err != nil {
			return PullResult{}, fmt.Errorf("sync: pull delta scan: %w", err)
		}
		c.Op = ChangeOp(op)
		out.Changes = append(out.Changes, c)
	}
	if err := rows.Err(); err != nil {
		return PullResult{}, fmt.Errorf("sync: pull delta rows: %w", err)
	}
	// Rows are seq-ascending, so the last is the high-water (the global max seq
	// is always the latest for its (entity,field), hence survives coalescing).
	if n := len(out.Changes); n > 0 {
		out.Cursor = out.Changes[n-1].Seq
	}
	return out, nil
}

// snapshot builds a full picture of the user's current workspace from the
// canonical tables (tree_nodes + artifacts), as create + field change rows
// stamped at the feed high-water. Served to a cold client (cursor == 0) so
// pre-feed entities are included; the client applies these idempotently via the
// per-(entity,field) seq guard, then continues with deltas from the high-water.
func (r *SyncRepo) snapshot(ctx context.Context, userID string) (PullResult, error) {
	var hw int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM workspace_changes WHERE user_id = $1`,
		userID).Scan(&hw); err != nil {
		return PullResult{}, fmt.Errorf("sync: snapshot high-water: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT n.id, n.kind, n.parent_id, n.position,
		       n.title AS node_title,
		       a.id AS artifact_id, a.type, a.title AS artifact_title,
		       a.content, a.summary, a.source_url
		  FROM tree_nodes n
		  LEFT JOIN artifacts a ON a.id = n.artifact_id AND a.user_id = n.user_id
		 WHERE n.user_id = $1::uuid
		 ORDER BY n.position ASC, n.created_at ASC`, userID)
	if err != nil {
		return PullResult{}, fmt.Errorf("sync: snapshot query: %w", err)
	}
	defer rows.Close()

	out := PullResult{Changes: []Change{}, Cursor: hw}
	for rows.Next() {
		var (
			nodeID, kind                   string
			parentID, nodeTitle            *string
			position                       int64
			artifactID, aType, aTitle      *string
			aContent, aSummary, aSourceURL *string
		)
		if err := rows.Scan(&nodeID, &kind, &parentID, &position, &nodeTitle,
			&artifactID, &aType, &aTitle, &aContent, &aSummary, &aSourceURL); err != nil {
			return PullResult{}, fmt.Errorf("sync: snapshot scan: %w", err)
		}

		isArtifact := kind == "artifact"
		entityID := nodeID
		title := nodeTitle
		if isArtifact {
			if artifactID == nil {
				continue // artifact node without a backing artifact row — skip
			}
			entityID = *artifactID
			title = aTitle
		}

		out.Changes = append(out.Changes, Change{Seq: hw, EntityKind: kind, EntityID: entityID, Op: OpCreate})
		if title != nil {
			out.Changes = append(out.Changes, snapField(kind, entityID, "title", *title, hw))
		}
		out.Changes = append(out.Changes, snapField(kind, entityID, "parentId", parentID, hw))
		out.Changes = append(out.Changes, snapField(kind, entityID, "position", position, hw))
		if isArtifact {
			if aType != nil {
				out.Changes = append(out.Changes, snapField(kind, entityID, "type", *aType, hw))
			}
			if aContent != nil && *aContent != "" {
				out.Changes = append(out.Changes, snapField(kind, entityID, "content", *aContent, hw))
			}
			if aSummary != nil {
				out.Changes = append(out.Changes, snapField(kind, entityID, "summary", *aSummary, hw))
			}
			if aSourceURL != nil {
				out.Changes = append(out.Changes, snapField(kind, entityID, "sourceUrl", *aSourceURL, hw))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return PullResult{}, fmt.Errorf("sync: snapshot rows: %w", err)
	}
	return out, nil
}

func snapField(kind, entityID, field string, value any, seq int64) Change {
	v, _ := json.Marshal(value)
	f := field
	return Change{Seq: seq, EntityKind: kind, EntityID: entityID, Op: OpUpdate, Field: &f, Value: v}
}
