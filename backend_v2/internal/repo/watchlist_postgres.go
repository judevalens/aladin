package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"aladin/backend_v2/internal/watchlist"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWatchlistRepository backs named watchlists (universes): a `watchlists` parent + its
// members (watchlist_items), joined to instruments for display. Ownership is enforced in every
// WHERE (user_id) plus a WHERE EXISTS guard on adds so a forged listID can't cross tenants.
type PostgresWatchlistRepository struct{ pool *pgxpool.Pool }

func NewWatchlistPostgres(pool *pgxpool.Pool) *PostgresWatchlistRepository {
	return &PostgresWatchlistRepository{pool: pool}
}

func (r *PostgresWatchlistRepository) ListWatchlists(ctx context.Context, userID string) ([]watchlist.Watchlist, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT wl.id::text, wl.name, wl.kind, wl.definition, wl.position,
		       COUNT(wi.instrument_id) AS item_count,
		       to_char(wl.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		  FROM watchlists wl
		  LEFT JOIN watchlist_items wi ON wi.watchlist_id = wl.id
		 WHERE wl.user_id = $1::uuid AND wl.is_deleted = false
		 GROUP BY wl.id
		 ORDER BY wl.position, wl.created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist list lists: %w", err)
	}
	defer rows.Close()
	out := make([]watchlist.Watchlist, 0)
	for rows.Next() {
		var w watchlist.Watchlist
		var def []byte
		if err := rows.Scan(&w.ID, &w.Name, &w.Kind, &def, &w.Position, &w.ItemCount, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("watchlist scan: %w", err)
		}
		if len(def) > 0 {
			w.Definition = json.RawMessage(def)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// CreateWatchlist inserts the list and emits its first upsert frame in one tx (LockUser →
// insert → emit → commit), so the durable data and the sync event commit together.
func (r *PostgresWatchlistRepository) CreateWatchlist(ctx context.Context, w watchlist.Watchlist, userID string) (watchlist.Watchlist, error) {
	def := w.Definition
	if len(def) == 0 {
		def = json.RawMessage(`{}`)
	}
	kind := w.Kind
	if kind == "" {
		kind = watchlist.Manual
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return watchlist.Watchlist{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return watchlist.Watchlist{}, err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO watchlists (id, user_id, name, kind, definition, position)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb, $6)
		RETURNING to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
	`, w.ID, userID, w.Name, kind, string(def), w.Position).Scan(&w.CreatedAt); err != nil {
		return watchlist.Watchlist{}, fmt.Errorf("watchlist create: %w", err)
	}
	if err := emitWatchlistUpsert(ctx, tx, userID, w.ID); err != nil {
		return watchlist.Watchlist{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return watchlist.Watchlist{}, fmt.Errorf("watchlist create commit: %w", err)
	}
	w.Kind = kind
	w.Definition = def
	return w, nil
}

func (r *PostgresWatchlistRepository) RenameWatchlist(ctx context.Context, userID, id, name string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE watchlists SET name = $3, updated_at = now()
		 WHERE id = $1::uuid AND user_id = $2::uuid AND is_deleted = false
	`, id, userID, name)
	if err != nil {
		return fmt.Errorf("watchlist rename: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return watchlist.ErrNotFound
	}
	if err := emitWatchlistUpsert(ctx, tx, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteWatchlist tombstones the list (soft delete → sync delete frame) and hard-removes its items
// (they are not independently synced; the list's Op:delete drops them client-side).
func (r *PostgresWatchlistRepository) DeleteWatchlist(ctx context.Context, userID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM watchlist_items WHERE watchlist_id = $1::uuid AND user_id = $2::uuid
	`, id, userID); err != nil {
		return fmt.Errorf("watchlist delete items: %w", err)
	}
	ok, err := emitWatchlistDelete(ctx, tx, userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return watchlist.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *PostgresWatchlistRepository) GetWatchlist(ctx context.Context, userID, id string) (watchlist.Watchlist, bool, error) {
	var w watchlist.Watchlist
	var def []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, name, kind, definition, position,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		  FROM watchlists WHERE id = $1::uuid AND user_id = $2::uuid AND is_deleted = false
	`, id, userID).Scan(&w.ID, &w.Name, &w.Kind, &def, &w.Position, &w.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return watchlist.Watchlist{}, false, nil
		}
		return watchlist.Watchlist{}, false, fmt.Errorf("watchlist get: %w", err)
	}
	if len(def) > 0 {
		w.Definition = json.RawMessage(def)
	}
	return w, true, nil
}

func (r *PostgresWatchlistRepository) DefaultWatchlistID(ctx context.Context, userID string) (string, bool, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text FROM watchlists
		 WHERE user_id = $1::uuid AND is_deleted = false
		 ORDER BY position, created_at LIMIT 1
	`, userID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("watchlist default id: %w", err)
	}
	return id, true, nil
}

func (r *PostgresWatchlistRepository) ListItems(ctx context.Context, userID, listID string) ([]watchlist.WatchlistItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.instrument_id::text, i.symbol, i.name, i.exchange,
		       to_char(w.added_at, 'YYYY-MM-DD') AS added_at
		  FROM watchlist_items w
		  JOIN instruments i ON i.instrument_id = w.instrument_id
		 WHERE w.watchlist_id = $1::uuid AND w.user_id = $2::uuid
		 ORDER BY w.added_at DESC
	`, listID, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist list items: %w", err)
	}
	defer rows.Close()
	var out []watchlist.WatchlistItem
	for rows.Next() {
		var it watchlist.WatchlistItem
		if err := rows.Scan(&it.InstrumentID, &it.Symbol, &it.Name, &it.Exchange, &it.AddedAt); err != nil {
			return nil, fmt.Errorf("watchlist item scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// AddItem inserts only if the target list belongs to the user and is live (WHERE EXISTS) — a forged
// listID from the API/MCP layer cannot add to another tenant's or a deleted list. Idempotent. A
// genuine insert re-emits the parent list's frame (its membership changed); an already-present add
// or a rejected forged listID changes nothing, so it emits nothing.
func (r *PostgresWatchlistRepository) AddItem(ctx context.Context, userID, listID, instrumentID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO watchlist_items (user_id, watchlist_id, instrument_id)
		SELECT $1::uuid, $2::uuid, $3::uuid
		 WHERE EXISTS (SELECT 1 FROM watchlists WHERE id = $2::uuid AND user_id = $1::uuid AND is_deleted = false)
		ON CONFLICT (watchlist_id, instrument_id) DO NOTHING
	`, userID, listID, instrumentID)
	if err != nil {
		return fmt.Errorf("watchlist add item: %w", err)
	}
	if tag.RowsAffected() > 0 {
		if err := emitWatchlistUpsert(ctx, tx, userID, listID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresWatchlistRepository) RemoveItem(ctx context.Context, userID, listID, instrumentID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		DELETE FROM watchlist_items
		 WHERE watchlist_id = $1::uuid AND user_id = $2::uuid AND instrument_id = $3::uuid
	`, listID, userID, instrumentID)
	if err != nil {
		return fmt.Errorf("watchlist remove item: %w", err)
	}
	if tag.RowsAffected() > 0 {
		if err := emitWatchlistUpsert(ctx, tx, userID, listID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
