package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeEmitter appends a node-change frame to a user's outbox out of band (its own
// tx), for producers that don't already hold a write tx — chiefly the async worker
// (pool-based claim writes, no principal in ctx). HTTP writes that already run in a
// tx call emitNodeUpsert inline instead.
type NodeEmitter struct{ pool *pgxpool.Pool }

func NewNodeEmitter(pool *pgxpool.Pool) *NodeEmitter { return &NodeEmitter{pool: pool} }

// EmitNodeChange bumps the artifact node's seq and appends an upsert frame to the
// owner's outbox, so the client refetches any derived view (e.g. the graph pane)
// on the resulting NodeUpserted DataEvent. `userID` is explicit — the worker has
// no principal in ctx. `tree_nodes.id == artifact_id`, so nodeID == artifactID.
// Idempotent from the client's view (the seq bump is a monotonic staleness guard),
// so a retry after a failed async task is safe.
func (e *NodeEmitter) EmitNodeChange(ctx context.Context, userID, artifactID string) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if err := emitNodeUpsert(ctx, tx, userID, artifactID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
