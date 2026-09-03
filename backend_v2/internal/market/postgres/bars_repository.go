package postgres

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/market"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBarRepository backs the global bar store, resolving symbols to instrument_id.
type PostgresBarRepository struct{ pool *pgxpool.Pool }

func NewBarPostgres(pool *pgxpool.Pool) *PostgresBarRepository {
	return &PostgresBarRepository{pool: pool}
}

// ListBars returns the latest `limit` bars for an active symbol+timeframe, oldest → newest
// (chart order). Joins instruments on the active-listing symbol.
func (r *PostgresBarRepository) ListBars(ctx context.Context, symbol, timeframe string, limit int) ([]market.Bar, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT b.ts, b.open, b.high, b.low, b.close, b.volume
		  FROM bars b
		  JOIN instruments i ON i.instrument_id = b.instrument_id
		 WHERE upper(i.symbol) = upper($1) AND i.is_active AND b.timeframe = $2
		 ORDER BY b.ts DESC
		 LIMIT $3
	`, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("bars list: %w", err)
	}
	defer rows.Close()
	out := make([]market.Bar, 0, limit)
	for rows.Next() {
		var b market.Bar
		if err := rows.Scan(&b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, fmt.Errorf("bars scan: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bars rows: %w", err)
	}
	// Reverse to ascending (query is DESC to hit the index + LIMIT).
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// UpsertBars writes bars idempotently, resolving symbol → active instrument_id once per
// distinct symbol. Bars for an unknown symbol are skipped. Batched.
func (r *PostgresBarRepository) UpsertBars(ctx context.Context, rows []market.BarUpsert) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	idBySymbol := map[string]string{}
	resolve := func(symbol string) (string, bool) {
		key := strings.ToUpper(symbol)
		if id, ok := idBySymbol[key]; ok {
			return id, id != ""
		}
		var id string
		err := r.pool.QueryRow(ctx,
			`SELECT instrument_id::text FROM instruments WHERE upper(symbol) = $1 AND is_active LIMIT 1`, key).Scan(&id)
		if err != nil {
			idBySymbol[key] = "" // negative cache: unknown symbol
			return "", false
		}
		idBySymbol[key] = id
		return id, true
	}

	batch := &pgx.Batch{}
	for _, u := range rows {
		id, ok := resolve(u.Symbol)
		if !ok {
			continue
		}
		batch.Queue(`
			INSERT INTO bars (instrument_id, timeframe, ts, open, high, low, close, volume)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (instrument_id, timeframe, ts)
			DO UPDATE SET open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
			              close = EXCLUDED.close, volume = EXCLUDED.volume
		`, id, u.Timeframe, u.Bar.Time, u.Bar.Open, u.Bar.High, u.Bar.Low, u.Bar.Close, u.Bar.Volume)
	}
	if batch.Len() == 0 {
		return 0, nil
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	written := 0
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return written, fmt.Errorf("bars upsert: %w", err)
		}
		written++
	}
	return written, nil
}
