package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Bar is one OHLCV bar as served to the chart. Raw/unadjusted; adjust-on-read comes later.
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

// BarService backs the ticker chart (read) and the bar backfill (sync).
type BarService interface {
	Get(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error)
	// SyncBars pulls history for a symbol from a source and upserts it; returns rows written.
	SyncBars(ctx context.Context, src BarSource, symbol, timeframe, start, end string) (int, error)
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
	repo   BarRepository
	source BarSource // optional vendor fallback; nil ⇒ local-only

	mu        sync.Mutex
	lastFetch map[string]time.Time // symbol|tf → last vendor fetch (throttle)
}

func NewBarService(repo BarRepository) *defaultBarService {
	return &defaultBarService{repo: repo, lastFetch: map[string]time.Time{}}
}

// WithSource enables read-through: on a local miss or stale series, Get fetches the gap from
// the vendor and upserts it. Returns the service for chaining.
func (s *defaultBarService) WithSource(source BarSource) *defaultBarService {
	s.source = source
	return s
}

func (s *defaultBarService) Get(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error) {
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
	return bars, nil
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
