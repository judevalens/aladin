package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresInstrumentRepository backs ticker search over the instruments registry.
type PostgresInstrumentRepository struct{ pool *pgxpool.Pool }

func NewInstrumentPostgres(pool *pgxpool.Pool) *PostgresInstrumentRepository {
	return &PostgresInstrumentRepository{pool: pool}
}

// SearchInstruments ranks matches for the command-box typeahead: an exact symbol beats a
// symbol prefix, which beats a name substring, which beats a trigram-fuzzy symbol/name.
// Active listings sort ahead of delisted ones so recycled tickers surface the live row.
func (r *PostgresInstrumentRepository) SearchInstruments(ctx context.Context, query string, limit int) ([]coreservice.InstrumentHit, error) {
	q := strings.TrimSpace(query)
	lower := strings.ToLower(q)
	prefix := lower + "%"
	like := "%" + lower + "%"

	rows, err := r.pool.Query(ctx, `
		SELECT instrument_id::text, symbol, name, exchange, asset_class, is_active,
		       CASE WHEN lower(symbol) = $1 THEN 4
		            WHEN lower(symbol) LIKE $2 THEN 3
		            WHEN lower(name) LIKE $3 THEN 2
		            ELSE 1 END AS tier,
		       GREATEST(similarity(symbol, $4), similarity(name, $4)) AS sim
		  FROM public.instruments
		 WHERE lower(symbol) LIKE $2
		    OR lower(name) LIKE $3
		    OR similarity(symbol, $4) >= 0.2
		 ORDER BY tier DESC, is_active DESC, sim DESC, symbol ASC
		 LIMIT $5
	`, lower, prefix, like, q, limit)
	if err != nil {
		return nil, fmt.Errorf("instrument search: %w", err)
	}
	defer rows.Close()

	hits := make([]coreservice.InstrumentHit, 0, limit)
	for rows.Next() {
		var h coreservice.InstrumentHit
		var tier int
		var sim float64
		if err := rows.Scan(&h.ID, &h.Symbol, &h.Name, &h.Exchange, &h.AssetClass, &h.IsActive, &tier, &sim); err != nil {
			return nil, fmt.Errorf("instrument search scan: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("instrument search rows: %w", err)
	}
	return hits, nil
}

// ResolveInstrumentID maps an active symbol to its stable instrument_id.
func (r *PostgresInstrumentRepository) ResolveInstrumentID(ctx context.Context, symbol string) (string, bool, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT instrument_id::text FROM instruments WHERE upper(symbol) = upper($1) AND is_active LIMIT 1`,
		strings.TrimSpace(symbol)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("instrument resolve id: %w", err)
	}
	return id, true, nil
}

// UpsertInstruments writes reference data idempotently, keyed on the active-listing symbol
// index (instruments_active_symbol_uq). Re-running is a no-op on unchanged rows. Batched so
// a full ~11k-symbol universe lands in one round trip. `name`/`exchange` from the vendor win;
// `cusip`/`cik`/`entity_id` are preserved (COALESCE) since the Assets API doesn't carry them.
func (r *PostgresInstrumentRepository) UpsertInstruments(ctx context.Context, rows []coreservice.InstrumentUpsert) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, in := range rows {
		symbol := strings.TrimSpace(in.Symbol)
		if symbol == "" {
			continue
		}
		assetClass := in.AssetClass
		if assetClass == "" {
			assetClass = "us_equity"
		}
		batch.Queue(`
			INSERT INTO public.instruments (symbol, name, exchange, asset_class, is_active)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (symbol) WHERE is_active
			DO UPDATE SET name        = EXCLUDED.name,
			              exchange     = EXCLUDED.exchange,
			              asset_class  = EXCLUDED.asset_class,
			              updated_at   = now()
		`, symbol, in.Name, in.Exchange, assetClass, in.IsActive)
	}
	if batch.Len() == 0 {
		return 0, nil
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	written := 0
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return written, fmt.Errorf("instrument upsert: %w", err)
		}
		written++
	}
	return written, nil
}
