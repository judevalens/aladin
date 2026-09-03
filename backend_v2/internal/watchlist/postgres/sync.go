package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"aladin/backend_v2/internal/outbox"
	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Data-layer R1 — the watchlist frame producer + cold-start snapshot.
//
// A watchlist is ONE synced entity (kind "watchlist"): its light `data` carries the list fields
// plus its ordered members, so an add/remove/rename all re-emit the SAME entity (its seq bumped).
// This mirrors the tree/signal producers — writes append a data_event frame in their own tx, the
// CDC drain republishes it as `*.frame`, and the client applies it by (kind, id, seq). Deletes
// tombstone the list (is_deleted, row kept) so a stale lower-seq upsert can't resurrect it.

const watchlistEntityKind = "watchlist"

type scanner interface{ Scan(dest ...any) error }

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type lightWatchlistItem struct {
	InstrumentID string `json:"instrumentId"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Position     int64  `json:"position"`
}

type lightWatchlistData struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Kind       string               `json:"kind"`
	Definition json.RawMessage      `json:"definition,omitempty"`
	Position   int64                `json:"position"`
	Items      []lightWatchlistItem `json:"items"`
}

// lightWatchlistSelect projects a watchlists row to its light columns, aggregating its members into
// an ordered items[] via a correlated subquery. $1 = user_id. Callers append a row filter
// (` AND w.id = $2::uuid`) or an ordering. It deliberately does NOT filter is_deleted — tombstones
// must reach the snapshot so the client seq guard blocks resurrection.
const lightWatchlistSelect = `
	SELECT w.id::text, w.name, w.kind, w.definition, w.position, w.seq, w.is_deleted,
	       COALESCE(
	         (SELECT json_agg(json_build_object(
	                    'instrumentId', i.instrument_id::text,
	                    'symbol',       i.symbol,
	                    'name',         i.name,
	                    'position',     wi.position)
	                  ORDER BY wi.position, wi.added_at)
	            FROM watchlist_items wi
	            JOIN instruments i ON i.instrument_id = wi.instrument_id
	           WHERE wi.watchlist_id = w.id),
	         '[]'::json) AS items
	  FROM watchlists w
	 WHERE w.user_id = $1::uuid`

// scanLightWatchlist maps one projected row to a FrameEntity: a tombstoned row → Op:delete (seq
// only, no data); a live row → Op:upsert with the list + its items[].
func scanLightWatchlist(row scanner) (service.FrameEntity, error) {
	var (
		d         lightWatchlistData
		def       []byte
		items     []byte
		seq       int64
		isDeleted bool
	)
	if err := row.Scan(&d.ID, &d.Name, &d.Kind, &def, &d.Position, &seq, &isDeleted, &items); err != nil {
		return service.FrameEntity{}, err
	}
	ent := service.FrameEntity{EntityKind: watchlistEntityKind, EntityID: d.ID, Seq: uint64(seq)}
	if isDeleted {
		ent.Op = service.OpDelete
		return ent, nil
	}
	ent.Op = service.OpUpsert
	if len(def) > 0 {
		d.Definition = json.RawMessage(def)
	}
	d.Items = []lightWatchlistItem{}
	if len(items) > 0 {
		if err := json.Unmarshal(items, &d.Items); err != nil {
			return service.FrameEntity{}, err
		}
	}
	data, err := json.Marshal(d)
	if err != nil {
		return service.FrameEntity{}, err
	}
	ent.Data = data
	return ent, nil
}

// lightWatchlistByID reads one list's current light projection (used by producers after a canonical
// mutation + seq bump, to build the frame entity).
func lightWatchlistByID(ctx context.Context, q rowQuerier, userID, listID string) (service.FrameEntity, error) {
	ent, err := scanLightWatchlist(q.QueryRow(ctx, lightWatchlistSelect+` AND w.id = $2::uuid`, userID, listID))
	if err != nil {
		return service.FrameEntity{}, fmt.Errorf("sync: light watchlist %s: %w", listID, err)
	}
	return ent, nil
}

// emitWatchlistUpsert is the common producer path for a create/rename/add/remove: bump the list's
// seq, read its light projection, append a single-entity upsert frame — all in the caller's write
// tx, so data + event commit together. Caller must hold LockUser for the user.
func emitWatchlistUpsert(ctx context.Context, tx pgx.Tx, userID, listID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE watchlists SET seq = seq + 1, updated_at = now()
		  WHERE id = $1::uuid AND user_id = $2::uuid`, listID, userID); err != nil {
		return fmt.Errorf("sync: bump watchlist seq %s: %w", listID, err)
	}
	ent, err := lightWatchlistByID(ctx, tx, userID, listID)
	if err != nil {
		return err
	}
	return outbox.AppendData(ctx, tx, userID, service.Frame{Entities: []service.FrameEntity{ent}})
}

// emitWatchlistDelete tombstones one list (is_deleted=true, seq bumped, row KEPT) and appends its
// delete frame. Returns ok=false if no live list matched (not found / already deleted) so the caller
// can map ErrWatchlistNotFound. Caller must hold LockUser for the user.
func emitWatchlistDelete(ctx context.Context, tx pgx.Tx, userID, listID string) (bool, error) {
	var seq int64
	err := tx.QueryRow(ctx, `
		UPDATE watchlists SET is_deleted = true, seq = seq + 1, updated_at = now()
		 WHERE id = $1::uuid AND user_id = $2::uuid AND is_deleted = false
		RETURNING seq`, listID, userID).Scan(&seq)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("sync: tombstone watchlist %s: %w", listID, err)
	}
	ent := service.FrameEntity{
		EntityKind: watchlistEntityKind,
		EntityID:   listID,
		Seq:        uint64(seq),
		Op:         service.OpDelete,
	}
	if err := outbox.AppendData(ctx, tx, userID, service.Frame{Entities: []service.FrameEntity{ent}}); err != nil {
		return false, err
	}
	return true, nil
}

// WatchlistSyncSource is the cold-start snapshot provider for watchlists (per-user, tombstones
// included). Implements service.SyncSource.
type SyncSource struct{ pool *pgxpool.Pool }

func NewSyncSource(pool *pgxpool.Pool) *SyncSource {
	return &SyncSource{pool: pool}
}

func (s *SyncSource) EntityKind() string { return watchlistEntityKind }

func (s *SyncSource) Snapshot(ctx context.Context, userID string) ([]service.FrameEntity, error) {
	rows, err := s.pool.Query(ctx, lightWatchlistSelect+` ORDER BY w.position, w.created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("sync: watchlist snapshot query: %w", err)
	}
	defer rows.Close()

	out := make([]service.FrameEntity, 0)
	for rows.Next() {
		ent, err := scanLightWatchlist(rows)
		if err != nil {
			return nil, fmt.Errorf("sync: watchlist snapshot scan: %w", err)
		}
		out = append(out, ent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sync: watchlist snapshot rows: %w", err)
	}
	return out, nil
}
