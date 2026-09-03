package market

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Bar is one OHLCV bar. STORED raw/unadjusted (TRADING_PRD §5); the corporate-actions log is
// replayed over it at read time — see bar_adjust.go.
type Bar struct {
	Time   time.Time `json:"t"`
	Open   float64   `json:"o"`
	High   float64   `json:"h"`
	Low    float64   `json:"l"`
	Close  float64   `json:"c"`
	Volume int64     `json:"v"`
}

// BarUpsert is one bar to persist, keyed on (symbol, timeframe, time). Symbol is resolved to
// instrument_id in the repo.
type BarUpsert struct {
	Symbol    string
	Timeframe string
	Bar       Bar
}

// BarRepository is the persistence port for the global bar store.
type BarRepository interface {
	// ListBars returns the latest `limit` bars for symbol+timeframe, oldest → newest.
	ListBars(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error)
	// UpsertBars writes bars idempotently; returns rows written.
	UpsertBars(ctx context.Context, rows []BarUpsert) (int, error)
}

// BarSource is a reference-data provider of historical bars (Alpaca), kept behind an
// interface so the sync is testable without a live vendor.
type BarSource interface {
	FetchBars(ctx context.Context, symbol, timeframe, start, end string) ([]Bar, error)
}

// CorporateActionRepository is the persistence port for the corporate-actions log.
type CorporateActionRepository interface {
	// ListActions returns a symbol's full action history, oldest ex-date first.
	ListActions(ctx context.Context, symbol string) ([]CorporateAction, error)
	// UpsertActions writes actions idempotently; returns rows written.
	UpsertActions(ctx context.Context, symbol string, actions []CorporateAction) (int, error)
}

// CorporateActionSource is a reference-data provider of corporate actions (Alpaca).
type CorporateActionSource interface {
	FetchCorporateActions(ctx context.Context, symbol, start, end string) ([]CorporateAction, error)
}

// BarService backs the ticker chart (read) and the bar backfill (sync).
type BarService interface {
	// Get returns bars SPLIT-ADJUSTED by default — raw storage would otherwise render a 4-for-1
	// split as a -75% crash. Use GetAdjusted for total-return (backtests) or raw.
	Get(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error)
	// GetAdjusted is Get with an explicit adjustment mode.
	GetAdjusted(ctx context.Context, symbol, timeframe string, limit int, mode AdjustMode) ([]Bar, error)
	// SyncBars pulls history for a symbol from a source and upserts it; returns rows written.
	SyncBars(ctx context.Context, src BarSource, symbol, timeframe, start, end string) (int, error)
	// SyncCorporateActions pulls a symbol's splits/dividends and upserts them; returns rows written.
	SyncCorporateActions(ctx context.Context, src CorporateActionSource, symbol, start, end string) (int, error)
}

const (
	barsDefaultLimit = 180
	barsMaxLimit     = 10000
	// A daily series is fresh if its newest bar is younger than this; otherwise Get lazily
	// fetches the gap from the vendor (read-through cache). Per-symbol fetches are throttled.
	barsStaleAfter       = 20 * time.Hour
	barsMinFetchInterval = 5 * time.Minute
)

type defaultBarService struct {
	repo    BarRepository
	source  BarSource                 // optional vendor fallback; nil ⇒ local-only
	actions CorporateActionRepository // optional; nil ⇒ bars are served raw

	mu        sync.Mutex
	lastFetch map[string]time.Time // symbol|tf → last vendor fetch (throttle)
}

func NewBarService(repo BarRepository) *defaultBarService {
	return &defaultBarService{repo: repo, lastFetch: map[string]time.Time{}}
}

// WithCorporateActions enables adjust-on-read. Without it the service serves raw bars (the
// pre-T1 behaviour), so wiring it is what makes split-crossing history correct.
func (s *defaultBarService) WithCorporateActions(actions CorporateActionRepository) *defaultBarService {
	s.actions = actions
	return s
}

// WithSource enables read-through: on a local miss or stale series, Get fetches the gap from
// the vendor and upserts it. Returns the service for chaining.
func (s *defaultBarService) WithSource(source BarSource) *defaultBarService {
	s.source = source
	return s
}

func (s *defaultBarService) Get(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error) {
	return s.GetAdjusted(ctx, symbol, timeframe, limit, AdjustSplits)
}

func (s *defaultBarService) GetAdjusted(ctx context.Context, symbol, timeframe string, limit int, mode AdjustMode) ([]Bar, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return []Bar{}, nil
	}
	if timeframe == "" {
		timeframe = "1Day"
	}
	if limit <= 0 {
		limit = barsDefaultLimit
	}
	if limit > barsMaxLimit {
		limit = barsMaxLimit
	}

	bars, err := s.repo.ListBars(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, err
	}
	// Read-through: if we can reach the vendor and the local cache is cold or stale, fetch the
	// gap and re-read. Best-effort — a vendor error falls back to whatever's cached.
	if s.source != nil && s.shouldFetch(symbol, timeframe, bars) {
		if s.fillFromVendor(ctx, symbol, timeframe, bars) {
			bars, err = s.repo.ListBars(ctx, symbol, timeframe, limit)
			if err != nil {
				return nil, err
			}
		}
	}

	// Adjust-on-read: replay the corporate-actions log over the raw series. Best-effort — if the
	// log can't be read we serve raw bars rather than failing the chart, but we say so, because
	// silently-unadjusted history is exactly the failure this design exists to prevent.
	if mode == AdjustNone || s.actions == nil || len(bars) == 0 {
		return bars, nil
	}
	actions, err := s.actions.ListActions(ctx, symbol)
	if err != nil {
		slog.Warn("bars: corporate actions unavailable; serving RAW bars",
			"component", "market", "symbol", symbol, "err", err)
		return bars, nil
	}
	return AdjustBars(bars, actions, mode), nil
}

// shouldFetch decides if a vendor read-through is warranted: cold (no bars) or stale newest,
// throttled to one fetch per symbol/timeframe per barsMinFetchInterval.
func (s *defaultBarService) shouldFetch(symbol, timeframe string, bars []Bar) bool {
	cold := len(bars) == 0
	stale := !cold && time.Since(bars[len(bars)-1].Time) > barsStaleAfter
	if !cold && !stale {
		return false
	}
	key := symbol + "|" + timeframe
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastFetch[key]) < barsMinFetchInterval {
		return false
	}
	s.lastFetch[key] = time.Now()
	return true
}

// fillFromVendor fetches [gap..now] and upserts. gap = the day after the newest cached bar,
// or ~2y back when cold. Returns true if any rows were written.
func (s *defaultBarService) fillFromVendor(ctx context.Context, symbol, timeframe string, bars []Bar) bool {
	start := time.Now().AddDate(-2, 0, 0)
	if len(bars) > 0 {
		start = bars[len(bars)-1].Time
	}
	fetched, err := s.source.FetchBars(ctx, symbol, timeframe, start.Format("2006-01-02"), "")
	if err != nil {
		slog.Warn("bars: vendor fetch failed", "component", "market", "symbol", symbol, "err", err)
		return false
	}
	if len(fetched) == 0 {
		return false
	}
	rows := make([]BarUpsert, 0, len(fetched))
	for _, b := range fetched {
		rows = append(rows, BarUpsert{Symbol: symbol, Timeframe: timeframe, Bar: b})
	}
	n, err := s.repo.UpsertBars(ctx, rows)
	if err != nil {
		slog.Warn("bars: upsert failed", "component", "market", "symbol", symbol, "err", err)
		return false
	}
	return n > 0
}

func (s *defaultBarService) SyncBars(ctx context.Context, src BarSource, symbol, timeframe, start, end string) (int, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return 0, nil
	}
	if timeframe == "" {
		timeframe = "1Day"
	}
	bars, err := src.FetchBars(ctx, symbol, timeframe, start, end)
	if err != nil {
		return 0, err
	}
	if len(bars) == 0 {
		return 0, nil
	}
	rows := make([]BarUpsert, 0, len(bars))
	for _, b := range bars {
		rows = append(rows, BarUpsert{Symbol: symbol, Timeframe: timeframe, Bar: b})
	}
	return s.repo.UpsertBars(ctx, rows)
}

func (s *defaultBarService) SyncCorporateActions(ctx context.Context, src CorporateActionSource, symbol, start, end string) (int, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" || s.actions == nil {
		return 0, nil
	}
	actions, err := src.FetchCorporateActions(ctx, symbol, start, end)
	if err != nil {
		return 0, err
	}
	if len(actions) == 0 {
		return 0, nil
	}
	return s.actions.UpsertActions(ctx, symbol, actions)
}
