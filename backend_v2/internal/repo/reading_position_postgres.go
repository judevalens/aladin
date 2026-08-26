package repo

import (
	"context"
	"errors"
	"fmt"

	"encoding/json"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reading position — the per-(user, artifact) "you are at page N" row on the R1
// sync spine (kind "reading_position", entity id = the artifact id; the outbox is
// user-scoped so no composite id). Writes are last-write-wins: PUT bumps seq in
// the same tx that appends the frame, so two devices scrolling simply converge on
// whichever page was reported last — a baseRevision guard would only force retry
// noise on a position report. Serialized per user via LockUser like every other
// producer. updated_at travels in the frame data as unix milliseconds so all
// three clients can compare it against their own session caches without parsing.

const readingPositionEntityKind = "reading_position"

type lightReadingPositionData struct {
	ArtifactID string `json:"artifactId"`
	Page       int64  `json:"page"`
	UpdatedAt  int64  `json:"updatedAt"` // unix ms
}

// lightReadingPositionSelect projects a user's rows for frames/snapshots.
// Tombstones are NOT filtered — they must reach the snapshot so the client seq
// guard blocks resurrection. $1 = user_id.
const lightReadingPositionSelect = `
	SELECT p.artifact_id, p.page, p.seq, p.is_deleted,
	       (extract(epoch FROM p.updated_at) * 1000)::bigint AS updated_at_ms
	  FROM reading_positions p
	 WHERE p.user_id = $1::uuid`

func scanLightReadingPosition(row scanner) (service.FrameEntity, error) {
	var (
		d         lightReadingPositionData
		seq       int64
		isDeleted bool
	)
	if err := row.Scan(&d.ArtifactID, &d.Page, &seq, &isDeleted, &d.UpdatedAt); err != nil {
		return service.FrameEntity{}, err
	}
	ent := service.FrameEntity{
		EntityKind: readingPositionEntityKind,
		EntityID:   d.ArtifactID,
		Seq:        uint64(seq),
	}
	if isDeleted {
		ent.Op = service.OpDelete
		return ent, nil
	}
	ent.Op = service.OpUpsert
	data, err := json.Marshal(d)
	if err != nil {
		return service.FrameEntity{}, err
	}
	ent.Data = data
	return ent, nil
}

// emitReadingPositionFrame reads one artifact's light projection (seq already
// final) and appends its frame in the caller's tx.
func emitReadingPositionFrame(ctx context.Context, tx pgx.Tx, userID, artifactID string) error {
	ent, err := scanLightReadingPosition(tx.QueryRow(ctx, lightReadingPositionSelect+` AND p.artifact_id = $2`, userID, artifactID))
	if err != nil {
		return fmt.Errorf("sync: light reading_position %s: %w", artifactID, err)
	}
	return appendOutboxEvent(ctx, tx, userID, service.Frame{Entities: []service.FrameEntity{ent}})
}

// ReadingPositionRepo implements service.ReadingPositionRepository on Postgres.
type ReadingPositionRepo struct{ pool *pgxpool.Pool }

func NewReadingPositionPostgres(pool *pgxpool.Pool) *ReadingPositionRepo {
	return &ReadingPositionRepo{pool: pool}
}

func scanReadingPosition(row pgx.Row) (service.ReadingPosition, error) {
	var p service.ReadingPosition
	err := row.Scan(&p.ArtifactID, &p.Page, &p.Seq, &p.UpdatedAt)
	return p, err
}

func (r *ReadingPositionRepo) GetReadingPosition(ctx context.Context, userID, artifactID string) (service.ReadingPosition, bool, error) {
	p, err := scanReadingPosition(r.pool.QueryRow(ctx, `
		SELECT artifact_id, page, seq, (extract(epoch FROM updated_at) * 1000)::bigint
		  FROM reading_positions
		 WHERE user_id = $1::uuid AND artifact_id = $2 AND is_deleted = false
	`, userID, artifactID))
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ReadingPosition{}, false, nil
	}
	if err != nil {
		return service.ReadingPosition{}, false, err
	}
	return p, true, nil
}

// PutReadingPosition upserts the row (LWW), bumps seq, and appends the frame in
// the same tx. The artifact must belong to the user.
func (r *ReadingPositionRepo) PutReadingPosition(ctx context.Context, userID, artifactID string, page int64) (service.ReadingPosition, error) {
	var out service.ReadingPosition
	err := r.withUserTx(ctx, userID, func(tx pgx.Tx) error {
		var owned bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM artifacts WHERE id = $1 AND user_id = $2::uuid)`,
			artifactID, userID).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return service.ErrReadingPositionNotFound
		}
		p, err := scanReadingPosition(tx.QueryRow(ctx, `
			INSERT INTO reading_positions (user_id, artifact_id, page, seq)
			VALUES ($1::uuid, $2, $3, 1)
			ON CONFLICT (user_id, artifact_id) DO UPDATE
			   SET page = EXCLUDED.page,
			       seq = reading_positions.seq + 1,
			       is_deleted = false,
			       updated_at = now()
			RETURNING artifact_id, page, seq, (extract(epoch FROM updated_at) * 1000)::bigint
		`, userID, artifactID, page))
		if err != nil {
			return err
		}
		out = p
		return emitReadingPositionFrame(ctx, tx, userID, artifactID)
	})
	return out, err
}

// withUserTx wraps fn in a tx holding the per-user advisory lock — the same
// serialization every outbox producer uses.
func (r *ReadingPositionRepo) withUserTx(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReadingPositionSyncSource is the cold-start snapshot provider (per-user,
// tombstones included). Implements service.SyncSource.
type ReadingPositionSyncSource struct{ pool *pgxpool.Pool }

func NewReadingPositionSyncSource(pool *pgxpool.Pool) *ReadingPositionSyncSource {
	return &ReadingPositionSyncSource{pool: pool}
}

func (s *ReadingPositionSyncSource) EntityKind() string { return readingPositionEntityKind }

func (s *ReadingPositionSyncSource) Snapshot(ctx context.Context, userID string) ([]service.FrameEntity, error) {
	rows, err := s.pool.Query(ctx, lightReadingPositionSelect+` ORDER BY p.artifact_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("sync: reading_position snapshot query: %w", err)
	}
	defer rows.Close()
	out := make([]service.FrameEntity, 0)
	for rows.Next() {
		ent, err := scanLightReadingPosition(rows)
		if err != nil {
			return nil, fmt.Errorf("sync: reading_position snapshot scan: %w", err)
		}
		out = append(out, ent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sync: reading_position snapshot rows: %w", err)
	}
	return out, nil
}
