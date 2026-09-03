package postgres

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/market"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCorporateActionRepository backs the corporate-actions log that adjust-on-read replays
// over raw bars (TRADING_PRD §5). Mirrors the bar repo: symbols are resolved to instrument_id at
// the boundary, because instrument_id is the stable identity and symbols get recycled.
type PostgresCorporateActionRepository struct{ pool *pgxpool.Pool }

func NewCorporateActionPostgres(pool *pgxpool.Pool) *PostgresCorporateActionRepository {
	return &PostgresCorporateActionRepository{pool: pool}
}

// ListActions returns every corporate action for an active symbol, oldest ex-date first.
// The whole history is returned deliberately: the adjustment is a cumulative replay, so a
// truncated log would silently under-adjust the oldest bars.
func (r *PostgresCorporateActionRepository) ListActions(ctx context.Context, symbol string) ([]market.CorporateAction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.type, c.ex_date, COALESCE(c.split_ratio, 0), COALESCE(c.cash_amount, 0)
		  FROM corporate_actions c
		  JOIN instruments i ON i.instrument_id = c.instrument_id
		 WHERE upper(i.symbol) = upper($1) AND i.is_active
		 ORDER BY c.ex_date ASC
	`, symbol)
	if err != nil {
		return nil, fmt.Errorf("corporate actions list: %w", err)
	}
	defer rows.Close()
	out := make([]market.CorporateAction, 0)
	for rows.Next() {
		var a market.CorporateAction
		if err := rows.Scan(&a.Type, &a.ExDate, &a.SplitRatio, &a.CashAmount); err != nil {
			return nil, fmt.Errorf("corporate actions scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("corporate actions rows: %w", err)
	}
	return out, nil
}

// UpsertActions writes actions idempotently for one symbol, keyed on
// (instrument_id, type, ex_date). Idempotence matters more here than for bars: a duplicated split
// would double-adjust every earlier bar. Unknown symbols are skipped.
func (r *PostgresCorporateActionRepository) UpsertActions(ctx context.Context, symbol string, actions []market.CorporateAction) (int, error) {
	if len(actions) == 0 {
		return 0, nil
	}
	var instrumentID string
	err := r.pool.QueryRow(ctx,
		`SELECT instrument_id::text FROM instruments WHERE upper(symbol) = $1 AND is_active LIMIT 1`,
		strings.ToUpper(strings.TrimSpace(symbol))).Scan(&instrumentID)
	if err != nil {
		return 0, nil // unknown symbol: nothing to attach the actions to
	}

	batch := &pgx.Batch{}
	for _, a := range actions {
		var ratio, cash any
		switch a.Type {
		case market.ActionSplit:
			if a.SplitRatio <= 0 {
				continue // the CHECK would reject it; skip rather than fail the batch
			}
			ratio = a.SplitRatio
		case market.ActionCashDividend:
			if a.CashAmount <= 0 {
				continue
			}
			cash = a.CashAmount
		default:
			continue
		}
		batch.Queue(`
			INSERT INTO corporate_actions (instrument_id, type, ex_date, split_ratio, cash_amount)
			VALUES ($1::uuid, $2, $3::date, $4, $5)
			ON CONFLICT (instrument_id, type, ex_date)
			DO UPDATE SET split_ratio = EXCLUDED.split_ratio, cash_amount = EXCLUDED.cash_amount
		`, instrumentID, a.Type, a.ExDate, ratio, cash)
	}
	if batch.Len() == 0 {
		return 0, nil
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	written := 0
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return written, fmt.Errorf("corporate actions upsert: %w", err)
		}
		written++
	}
	return written, nil
}
