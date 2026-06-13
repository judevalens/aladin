package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ShardBuildRepo persists shard_build_state and emits the build-status app_event
// transactionally (same tx as the row write), so a build-status change reaches
// the API process's websocket from the MCP build process. Implements
// service.ShardBuildStore.
type ShardBuildRepo struct {
	pool *pgxpool.Pool
}

func NewShardBuildPostgres(pool *pgxpool.Pool) *ShardBuildRepo {
	return &ShardBuildRepo{pool: pool}
}

func (r *ShardBuildRepo) SetStatus(ctx context.Context, st service.ShardBuildState) error {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	userID := principal.UserID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}

	var errorsJSON []byte
	if st.Errors != "" {
		if errorsJSON, err = json.Marshal(st.Errors); err != nil {
			return err
		}
	}
	var builtAt *time.Time
	if st.BuiltAt != "" {
		if t, perr := time.Parse(time.RFC3339, st.BuiltAt); perr == nil {
			builtAt = &t
		}
	}
	var buildID *string
	if st.BuildID != "" {
		buildID = &st.BuildID
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO shard_build_state (page_id, channel, status, errors, build_id, built_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, now())
		ON CONFLICT (page_id, channel) DO UPDATE SET
		  status     = EXCLUDED.status,
		  errors     = EXCLUDED.errors,
		  build_id   = EXCLUDED.build_id,
		  built_at   = EXCLUDED.built_at,
		  updated_at = now()
	`, st.PageID, string(st.Channel), string(st.Status), nullableBytes(errorsJSON), buildID, builtAt); err != nil {
		return err
	}

	// Emit the build-status event in the same tx — crosses to the API ws.
	payload, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := appendAppEvent(ctx, tx, userID, service.OutboxAppEvent{
		ResourceKind: "artifact",
		ResourceID:   st.PageID,
		Operation:    "build-status",
		Payload:      payload,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ShardBuildRepo) GetStatus(ctx context.Context, pageID string, channel service.BuildChannel) (service.ShardBuildState, error) {
	var (
		status     string
		errorsJSON []byte
		buildID    *string
		builtAt    *time.Time
	)
	err := r.pool.QueryRow(ctx, `
		SELECT status, errors, build_id, built_at
		  FROM shard_build_state
		 WHERE page_id = $1 AND channel = $2
	`, pageID, string(channel)).Scan(&status, &errorsJSON, &buildID, &builtAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// No build recorded yet — a zero-status state, not an error.
		return service.ShardBuildState{PageID: pageID, Channel: channel}, nil
	}
	if err != nil {
		return service.ShardBuildState{}, err
	}

	out := service.ShardBuildState{PageID: pageID, Channel: channel, Status: service.ShardBuildStatus(status)}
	if len(errorsJSON) > 0 {
		_ = json.Unmarshal(errorsJSON, &out.Errors)
	}
	if buildID != nil {
		out.BuildID = *buildID
	}
	if builtAt != nil {
		out.BuiltAt = builtAt.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// nullableBytes maps an empty slice to a nil interface so the column stores SQL
// NULL rather than an empty-string jsonb.
func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
