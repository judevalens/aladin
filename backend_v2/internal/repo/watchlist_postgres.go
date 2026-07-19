package repo

import (
	"context"
	"fmt"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWatchlistRepository backs the per-user watchlist, joined to instruments for
// display (symbol/name/exchange).
type PostgresWatchlistRepository struct{ pool *pgxpool.Pool }

func NewWatchlistPostgres(pool *pgxpool.Pool) *PostgresWatchlistRepository {
	return &PostgresWatchlistRepository{pool: pool}
}

func (r *PostgresWatchlistRepository) ListWatchlist(ctx context.Context, userID string) ([]coreservice.WatchlistItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.instrument_id::text, i.symbol, i.name, i.exchange,
		       to_char(w.added_at, 'YYYY-MM-DD') AS added_at
		  FROM watchlist_items w
		  JOIN instruments i ON i.instrument_id = w.instrument_id
		 WHERE w.user_id = $1::uuid
		 ORDER BY w.added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist list: %w", err)
	}
	defer rows.Close()
	var out []coreservice.WatchlistItem
	for rows.Next() {
		var it coreservice.WatchlistItem
		if err := rows.Scan(&it.InstrumentID, &it.Symbol, &it.Name, &it.Exchange, &it.AddedAt); err != nil {
			return nil, fmt.Errorf("watchlist scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// AddToWatchlist is idempotent — re-adding a tracked ticker is a no-op.
func (r *PostgresWatchlistRepository) AddToWatchlist(ctx context.Context, userID, instrumentID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO watchlist_items (user_id, instrument_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (user_id, instrument_id) DO NOTHING
	`, userID, instrumentID)
	if err != nil {
		return fmt.Errorf("watchlist add: %w", err)
	}
	return nil
}

func (r *PostgresWatchlistRepository) RemoveFromWatchlist(ctx context.Context, userID, instrumentID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM watchlist_items WHERE user_id = $1::uuid AND instrument_id = $2::uuid
	`, userID, instrumentID)
	if err != nil {
		return fmt.Errorf("watchlist remove: %w", err)
	}
	return nil
}
