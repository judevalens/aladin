package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Data-layer redesign, Phase B — the generic, idempotent write path.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
//
// A client pushes intents as Mutations to POST /api/sync/push. ApplyMutation
// applies one mutation to the canonical store (tree_nodes + artifacts) and
// appends its change-feed rows, all in ONE transaction under the per-user
// advisory lock (so the appended seq is visible in commit order). Idempotency
// is per-client high-water: mutationId = "clientId:counter"; a mutation is
// applied iff counter > sync_clients.last_applied for that client, then the
// high-water advances in the same txn (review F1 — a retried/duplicate write
// is a no-op, so it can't re-apply a stale value over a newer peer write).
//
// The canonical model splits a logical node across two tables: folders live in
// tree_nodes; an artifact's content lives in artifacts (title canonical there)
// and its placement (parent/position) in its tree_nodes row. ApplyMutation
// routes each field accordingly. This is the eventual single write path; the
// legacy typed endpoints stay until cutover.

// Mutation is one client intent. For op="update" it is a generic setField
// (field + value). For op="create" it carries the initial fields needed to
// satisfy the canonical tables' constraints (a self-contained create, grouped
// atomically by mutationId). For op="delete" only entity identity is needed.
type Mutation struct {
	MutationID string          `json:"mutationId"`
	Op         ChangeOp        `json:"op"`
	EntityKind string          `json:"entityKind"` // "folder" | "artifact"
	EntityID   string          `json:"entityId"`
	Field      *string         `json:"field,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`

	// create payload (ignored for update/delete)
	ParentID     *string `json:"parentId,omitempty"`
	Position     *int64  `json:"position,omitempty"`
	Title        *string `json:"title,omitempty"`
	ArtifactType *string `json:"artifactType,omitempty"`
	Content      *string `json:"content,omitempty"`
	Summary      *string `json:"summary,omitempty"`
	SourceURL    *string `json:"sourceUrl,omitempty"`
}

// MutationAck reports the outcome of one mutation back to the client.
type MutationAck struct {
	MutationID string `json:"mutationId"`
	Status     string `json:"status"` // "applied" | "duplicate"
}

const (
	ackApplied   = "applied"
	ackDuplicate = "duplicate"
)

// ApplyMutation applies one mutation idempotently and returns its ack.
func (r *SyncRepo) ApplyMutation(ctx context.Context, userID string, m Mutation) (MutationAck, error) {
	clientID, counter, err := parseMutationID(m.MutationID)
	if err != nil {
		return MutationAck{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return MutationAck{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := LockUser(ctx, tx, userID); err != nil {
		return MutationAck{}, err
	}

	// Idempotency high-water: skip anything we've already applied for this client.
	applied, err := claimCounter(ctx, tx, clientID, userID, counter)
	if err != nil {
		return MutationAck{}, err
	}
	if !applied {
		// Already applied (duplicate/retry) — commit the (no-op) lock txn so the
		// advisory lock releases cleanly, and ack as a duplicate.
		if err := tx.Commit(ctx); err != nil {
			return MutationAck{}, err
		}
		return MutationAck{MutationID: m.MutationID, Status: ackDuplicate}, nil
	}

	if err := applyOne(ctx, tx, userID, m); err != nil {
		return MutationAck{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MutationAck{}, err
	}
	return MutationAck{MutationID: m.MutationID, Status: ackApplied}, nil
}

// ChangesByMutation returns the change-feed rows a single mutation produced
// (they share its mutation_id), in seq order, so the push handler can echo them
// over the realtime stream for direct apply by peers.
func (r *SyncRepo) ChangesByMutation(ctx context.Context, userID, mutationID string) ([]Change, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT seq, entity_kind, entity_id, op, field, value, mutation_id
		  FROM workspace_changes
		 WHERE user_id = $1 AND mutation_id = $2
		 ORDER BY seq`, userID, mutationID)
	if err != nil {
		return nil, fmt.Errorf("sync: changes by mutation: %w", err)
	}
	defer rows.Close()
	var out []Change
	for rows.Next() {
		var c Change
		var op string
		if err := rows.Scan(&c.Seq, &c.EntityKind, &c.EntityID, &op, &c.Field, &c.Value, &c.MutationID); err != nil {
			return nil, err
		}
		c.Op = ChangeOp(op)
		out = append(out, c)
	}
	return out, rows.Err()
}

// claimCounter advances the per-client high-water iff counter is newer; returns
// false (skip) for a duplicate/old counter. Runs inside the caller's txn.
func claimCounter(ctx context.Context, tx pgx.Tx, clientID, userID string, counter int64) (bool, error) {
	var lastApplied int64
	err := tx.QueryRow(ctx,
		`SELECT last_applied FROM sync_clients WHERE client_id = $1`, clientID).Scan(&lastApplied)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// First write from this client.
		if _, err := tx.Exec(ctx, `
			INSERT INTO sync_clients (client_id, user_id, last_applied, updated_at)
			VALUES ($1, $2::uuid, $3, now())`, clientID, userID, counter); err != nil {
			return false, err
		}
		return true, nil
	case err != nil:
		return false, err
	}
	if counter <= lastApplied {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sync_clients SET last_applied = $2, updated_at = now()
		 WHERE client_id = $1`, clientID, counter); err != nil {
		return false, err
	}
	return true, nil
}

func applyOne(ctx context.Context, tx pgx.Tx, userID string, m Mutation) error {
	switch m.Op {
	case OpCreate:
		return applyCreate(ctx, tx, userID, m)
	case OpUpdate:
		return applyUpdate(ctx, tx, userID, m)
	case OpDelete:
		return applyDelete(ctx, tx, userID, m)
	default:
		return fmt.Errorf("sync: unknown op %q", m.Op)
	}
}

func applyCreate(ctx context.Context, tx pgx.Tx, userID string, m Mutation) error {
	// Assign the sibling position AUTHORITATIVELY (next free slot under the
	// per-user advisory lock), not from the client's hint. The client computes
	// its hint from local state that may be incomplete (e.g. no snapshot
	// backfill of pre-existing nodes), so trusting it collides with the unique
	// (user, parent, position) index. The client learns the real position from
	// the emitted "position" change row. m.Position is ignored.
	var position int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), 0) + 1 FROM tree_nodes
		 WHERE user_id = $1::uuid AND parent_id IS NOT DISTINCT FROM $2`,
		userID, m.ParentID).Scan(&position); err != nil {
		return err
	}
	switch m.EntityKind {
	case "folder":
		if _, err := tx.Exec(ctx, `
			INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
			VALUES ($1, $2::uuid, $3, 'folder', $4, NULL, $5, now(), now())
			ON CONFLICT (id) DO NOTHING
		`, m.EntityID, userID, m.ParentID, m.Title, position); err != nil {
			return err
		}
	case "artifact":
		artifactType := "note"
		if m.ArtifactType != nil && strings.TrimSpace(*m.ArtifactType) != "" {
			artifactType = *m.ArtifactType
		}
		title := ""
		if m.Title != nil {
			title = *m.Title
		}
		content := "" // artifacts.content is NOT NULL
		if m.Content != nil {
			content = *m.Content
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO artifacts (id, user_id, type, title, content, summary, source_url, metadata, created_at, updated_at)
			VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, '{}'::jsonb, now(), now())
			ON CONFLICT (id) DO NOTHING
		`, m.EntityID, userID, artifactType, title, content, m.Summary, m.SourceURL); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
			VALUES ($1, $2::uuid, $3, 'artifact', NULL, $1, $4, now(), now())
			ON CONFLICT (id) DO NOTHING
		`, m.EntityID, userID, m.ParentID, position); err != nil {
			return err
		}
	default:
		return fmt.Errorf("sync: unknown entity kind %q", m.EntityKind)
	}

	// Emit create + the initial fields (mirrors the legacy create path).
	if _, err := AppendChange(ctx, tx, userID, Change{
		EntityKind: m.EntityKind, EntityID: m.EntityID, Op: OpCreate, MutationID: &m.MutationID,
	}); err != nil {
		return err
	}
	if m.Title != nil {
		if err := appendFieldMut(ctx, tx, userID, m.EntityKind, m.EntityID, "title", *m.Title, m.MutationID); err != nil {
			return err
		}
	}
	if m.EntityKind == "artifact" {
		artifactType := "note"
		if m.ArtifactType != nil && strings.TrimSpace(*m.ArtifactType) != "" {
			artifactType = *m.ArtifactType
		}
		if err := appendFieldMut(ctx, tx, userID, "artifact", m.EntityID, "type", artifactType, m.MutationID); err != nil {
			return err
		}
	}
	if err := appendFieldMut(ctx, tx, userID, m.EntityKind, m.EntityID, "parentId", m.ParentID, m.MutationID); err != nil {
		return err
	}
	return appendFieldMut(ctx, tx, userID, m.EntityKind, m.EntityID, "position", position, m.MutationID)
}

func applyUpdate(ctx context.Context, tx pgx.Tx, userID string, m Mutation) error {
	if m.Field == nil {
		return fmt.Errorf("sync: update mutation missing field")
	}
	field := *m.Field
	switch m.EntityKind {
	case "folder":
		if err := applyFolderField(ctx, tx, userID, m.EntityID, field, m.Value); err != nil {
			return err
		}
	case "artifact":
		if err := applyArtifactField(ctx, tx, userID, m.EntityID, field, m.Value); err != nil {
			return err
		}
	default:
		return fmt.Errorf("sync: unknown entity kind %q", m.EntityKind)
	}
	// Echo the field change verbatim into the feed (value already JSON).
	f := field
	_, err := AppendChange(ctx, tx, userID, Change{
		EntityKind: m.EntityKind, EntityID: m.EntityID, Op: OpUpdate,
		Field: &f, Value: m.Value, MutationID: &m.MutationID,
	})
	return err
}

func applyFolderField(ctx context.Context, tx pgx.Tx, userID, id, field string, value json.RawMessage) error {
	switch field {
	case "title":
		title, err := decodeString(value)
		if err != nil {
			return err
		}
		return execTreeNodeUpdate(ctx, tx, userID, id, "folder",
			`UPDATE tree_nodes SET title = $3, updated_at = now() WHERE id = $1 AND user_id = $2::uuid AND kind = 'folder'`, title)
	case "parentId":
		parent, err := decodeNullableString(value)
		if err != nil {
			return err
		}
		return execTreeNodeUpdate(ctx, tx, userID, id, "folder",
			`UPDATE tree_nodes SET parent_id = $3, updated_at = now() WHERE id = $1 AND user_id = $2::uuid AND kind = 'folder'`, parent)
	case "position":
		pos, err := decodeInt(value)
		if err != nil {
			return err
		}
		return execTreeNodeUpdate(ctx, tx, userID, id, "folder",
			`UPDATE tree_nodes SET position = $3, updated_at = now() WHERE id = $1 AND user_id = $2::uuid AND kind = 'folder'`, pos)
	default:
		return fmt.Errorf("sync: unknown folder field %q", field)
	}
}

func applyArtifactField(ctx context.Context, tx pgx.Tx, userID, id, field string, value json.RawMessage) error {
	// Placement fields live on the artifact's tree_node; content fields on the
	// artifact row.
	switch field {
	case "parentId":
		parent, err := decodeNullableString(value)
		if err != nil {
			return err
		}
		return execTreeNodeUpdate(ctx, tx, userID, id, "artifact",
			`UPDATE tree_nodes SET parent_id = $3, updated_at = now() WHERE artifact_id = $1 AND user_id = $2::uuid AND kind = 'artifact'`, parent)
	case "position":
		pos, err := decodeInt(value)
		if err != nil {
			return err
		}
		return execTreeNodeUpdate(ctx, tx, userID, id, "artifact",
			`UPDATE tree_nodes SET position = $3, updated_at = now() WHERE artifact_id = $1 AND user_id = $2::uuid AND kind = 'artifact'`, pos)
	}

	var column string
	switch field {
	case "title":
		column = "title"
	case "type":
		column = "type"
	case "content":
		column = "content"
	case "summary":
		column = "summary"
	case "sourceUrl":
		column = "source_url"
	case "metadata":
		// metadata is JSONB; pass the raw value through.
		tag, err := tx.Exec(ctx, `
			UPDATE artifacts SET metadata = $3::jsonb, updated_at = now()
			 WHERE id = $1 AND user_id = $2::uuid`, id, userID, string(value))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFoundf("artifact %s", id)
		}
		return nil
	default:
		return fmt.Errorf("sync: unknown artifact field %q", field)
	}

	str, err := decodeString(value)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE artifacts SET %s = $3, updated_at = now()
		 WHERE id = $1 AND user_id = $2::uuid`, column), id, userID, str)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFoundf("artifact %s", id)
	}
	return nil
}

func applyDelete(ctx context.Context, tx pgx.Tx, userID string, m Mutation) error {
	// Resolve the node id: an artifact's tree_node is keyed by artifact_id.
	var nodeID string
	if m.EntityKind == "artifact" {
		err := tx.QueryRow(ctx,
			`SELECT id FROM tree_nodes WHERE artifact_id = $1 AND user_id = $2::uuid AND kind = 'artifact'`,
			m.EntityID, userID).Scan(&nodeID)
		if errors.Is(err, pgx.ErrNoRows) {
			// No node — delete the bare artifact + tombstone (idempotent).
			_, _ = tx.Exec(ctx, `DELETE FROM artifacts WHERE id = $1 AND user_id = $2::uuid`, m.EntityID, userID)
			_, err := AppendChange(ctx, tx, userID, Change{
				EntityKind: "artifact", EntityID: m.EntityID, Op: OpDelete, MutationID: &m.MutationID,
			})
			return err
		}
		if err != nil {
			return err
		}
	} else {
		nodeID = m.EntityID
	}

	// Gather the subtree (root + descendants) so we can tombstone each entity.
	rows, err := tx.Query(ctx, `
		WITH RECURSIVE subtree AS (
		    SELECT id, kind, artifact_id FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid
		    UNION ALL
		    SELECT n.id, n.kind, n.artifact_id FROM tree_nodes n
		      JOIN subtree s ON n.parent_id = s.id WHERE n.user_id = $2::uuid
		)
		SELECT id, kind, artifact_id FROM subtree`, nodeID, userID)
	if err != nil {
		return err
	}
	type subnode struct {
		id         string
		kind       string
		artifactID *string
	}
	var nodes []subnode
	for rows.Next() {
		var n subnode
		if err := rows.Scan(&n.id, &n.kind, &n.artifactID); err != nil {
			rows.Close()
			return err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(nodes) == 0 {
		// Already gone — emit a tombstone so peers converge, idempotently.
		_, err := AppendChange(ctx, tx, userID, Change{
			EntityKind: m.EntityKind, EntityID: m.EntityID, Op: OpDelete, MutationID: &m.MutationID,
		})
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid`, nodeID, userID); err != nil {
		return err
	}
	for _, n := range nodes {
		if n.artifactID != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM artifacts WHERE id = $1 AND user_id = $2::uuid`, *n.artifactID, userID); err != nil {
				return err
			}
		}
	}
	for _, n := range nodes {
		entityKind := "folder"
		entityID := n.id
		if n.kind == "artifact" {
			entityKind = "artifact"
			if n.artifactID != nil {
				entityID = *n.artifactID
			}
		}
		if _, err := AppendChange(ctx, tx, userID, Change{
			EntityKind: entityKind, EntityID: entityID, Op: OpDelete, MutationID: &m.MutationID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func execTreeNodeUpdate(ctx context.Context, tx pgx.Tx, userID, id, kind, query string, arg any) error {
	tag, err := tx.Exec(ctx, query, id, userID, arg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFoundf("%s %s", kind, id)
	}
	return nil
}

// appendFieldMut is appendField with a mutation id attached (transaction group).
func appendFieldMut(ctx context.Context, tx pgx.Tx, userID, kind, entityID, field string, value any, mutationID string) error {
	v, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f := field
	_, err = AppendChange(ctx, tx, userID, Change{
		EntityKind: kind, EntityID: entityID, Op: OpUpdate, Field: &f, Value: v, MutationID: &mutationID,
	})
	return err
}

// parseMutationID splits "clientId:counter" into its parts.
func parseMutationID(id string) (string, int64, error) {
	i := strings.LastIndex(id, ":")
	if i <= 0 || i == len(id)-1 {
		return "", 0, fmt.Errorf("sync: malformed mutationId %q (want clientId:counter)", id)
	}
	counter, err := strconv.ParseInt(id[i+1:], 10, 64)
	if err != nil || counter <= 0 {
		return "", 0, fmt.Errorf("sync: malformed mutationId counter in %q", id)
	}
	return id[:i], counter, nil
}

func decodeString(value json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(value, &s); err != nil {
		return "", fmt.Errorf("sync: expected string value: %w", err)
	}
	return s, nil
}

func decodeNullableString(value json.RawMessage) (*string, error) {
	if len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(value, &s); err != nil {
		return nil, fmt.Errorf("sync: expected string|null value: %w", err)
	}
	return &s, nil
}

func decodeInt(value json.RawMessage) (int64, error) {
	var n int64
	if err := json.Unmarshal(value, &n); err != nil {
		return 0, fmt.Errorf("sync: expected integer value: %w", err)
	}
	return n, nil
}

func errNotFoundf(format string, args ...any) error {
	return fmt.Errorf("sync: not found: "+format, args...)
}
