package market

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aladin/backend_v2/internal/market/alpaca"
	"aladin/backend_v2/internal/safego"
)

// MarketDataService is the demand-driven quote hub. Clients subscribe to symbols; the hub
// refcounts and drives ONE upstream Alpaca WS (subscribing a symbol only when its refcount
// goes 0→1, unsubscribing at 1→0). Upstream ticks are throttled and published to the outbox
// as broadcast 'market' quote events, which the drainer fans out to every subscriber.
type MarketDataService interface {
	Subscribe(ctx context.Context, symbols []string) error
	Unsubscribe(ctx context.Context, symbols []string) error
	// Start runs the upstream stream until ctx is cancelled (no-op without Alpaca keys).
	Start(ctx context.Context)
}

// InstrumentResolver maps a symbol to its stable instrument_id (satisfied by InstrumentService).
type InstrumentResolver interface {
	ResolveInstrumentID(ctx context.Context, symbol string) (string, bool, error)
}

// upstreamStream is the fan-in port — the single Alpaca WS. An interface so the hub is
// testable (refcount dedup) with a fake. *alpaca.Stream satisfies it.
type upstreamStream interface {
	Subscribe(ctx context.Context, symbols ...string)
	Unsubscribe(ctx context.Context, symbols ...string)
	Run(ctx context.Context)
}

const quoteThrottle = time.Second // ≤1 published quote per symbol per second

type marketDataHub struct {
	quotes    QuoteService
	resolver  InstrumentResolver
	snapshots QuoteSnapshotSource // optional: seed a real last-known quote on subscribe
	stream    upstreamStream      // nil when Alpaca isn't configured

	// tickObserver is an optional non-blocking tap on EVERY raw tick (before the throttle) —
	// the alert engine's hook. atomic.Value so onTrade reads it lock-free on the WS goroutine
	// while Set runs once at startup.
	tickObserver atomic.Value // func(symbol string, price float64, at time.Time)

	mu          sync.Mutex
	refcount    map[string]int       // symbol → active subscriptions
	idBySymbol  map[string]string    // symbol → instrument_id (cached at subscribe)
	lastPublish map[string]time.Time // symbol → last publish (throttle)

	// seedSem bounds concurrent snapshot-seed REST calls so a large subscribe (a big
	// watchlist/screener) can't fan out 100+ simultaneous Alpaca calls and trip a 429.
	seedSem chan struct{}
}

// seedSnapshotTimeout bounds a single snapshot-seed REST call (the Alpaca client's own 30s is the
// hard ceiling; this fails a stalled seed sooner and lets the goroutine exit).
const seedSnapshotTimeout = 15 * time.Second

// TickObserver receives every raw tick (before the 1/sec quote throttle). It MUST NOT block —
// it runs on the single WS read goroutine.
type TickObserver func(symbol string, price float64, at time.Time)

// SetTickObserver installs the tap (the alert engine). Call once at startup.
func (h *marketDataHub) SetTickObserver(fn TickObserver) { h.tickObserver.Store(fn) }

// NewMarketDataService builds the hub and its upstream Alpaca stream. When !configured the
// stream is nil: Subscribe/Unsubscribe still refcount, but nothing streams (degrades cleanly).
func NewMarketDataService(cfg MarketDataConfig, quotes QuoteService, resolver InstrumentResolver, snapshots QuoteSnapshotSource) MarketDataService {
	h := newHub(quotes, resolver)
	h.snapshots = snapshots
	if cfg.Configured {
		h.stream = alpaca.NewStream(cfg.StreamBaseURL, cfg.Feed, cfg.APIKey, cfg.APISecret, h.onTrade)
	}
	return h
}

// newHub builds a hub with no stream (the injectable core, shared by the real constructor
// and tests).
func newHub(quotes QuoteService, resolver InstrumentResolver) *marketDataHub {
	return &marketDataHub{
		quotes:      quotes,
		resolver:    resolver,
		refcount:    map[string]int{},
		idBySymbol:  map[string]string{},
		lastPublish: map[string]time.Time{},
		seedSem:     make(chan struct{}, 8),
	}
}

// MarketDataConfig carries the Alpaca stream settings (from config.AlpacaConfig) without a
// config import in the service layer.
type MarketDataConfig struct {
	Configured    bool
	StreamBaseURL string
	Feed          string
	APIKey        string
	APISecret     string
}

func (h *marketDataHub) Start(ctx context.Context) {
	if h.stream != nil {
		// Supervised: a panic in the WS read/reconnect loop must not silently kill live quotes.
		safego.Loop(ctx, "market.stream", h.stream.Run)
	}
}

func (h *marketDataHub) Subscribe(ctx context.Context, symbols []string) error {
	fresh := make([]string, 0, len(symbols))
	h.mu.Lock()
	for _, raw := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(raw))
		if sym == "" {
			continue
		}
		h.refcount[sym]++
		if h.refcount[sym] == 1 {
			fresh = append(fresh, sym)
		}
	}
	h.mu.Unlock()

	// Resolve + cache instrument ids for the newly-demanded symbols, then subscribe upstream.
	for _, sym := range fresh {
		if id, ok, err := h.resolver.ResolveInstrumentID(ctx, sym); err == nil && ok {
			h.mu.Lock()
			h.idBySymbol[sym] = id
			h.mu.Unlock()
		}
	}
	if h.stream != nil && len(fresh) > 0 {
		h.stream.Subscribe(ctx, fresh...)
	}
	// Seed a real last-known quote immediately (works off-hours), so the UI shows a live price
	// before the first WS tick. Async — never blocks the subscribe call.
	if h.snapshots != nil {
		for _, sym := range fresh {
			sym := sym
			safego.Go("market.seed", func() { h.seedSnapshot(sym) })
		}
	}
	return nil
}

func (h *marketDataHub) seedSnapshot(symbol string) {
	h.mu.Lock()
	id := h.idBySymbol[symbol]
	h.mu.Unlock()
	if id == "" {
		return
	}
	// Bound concurrent Alpaca REST seeds (avoid a 429 burst on a large subscribe).
	h.seedSem <- struct{}{}
	defer func() { <-h.seedSem }()

	ctx, cancel := context.WithTimeout(context.Background(), seedSnapshotTimeout)
	defer cancel()
	q, ok, err := h.snapshots.FetchSnapshot(ctx, symbol)
	if err != nil || !ok {
		return
	}
	q.Symbol = symbol
	q.InstrumentID = id
	_ = h.quotes.Publish(ctx, q)
}

func (h *marketDataHub) Unsubscribe(ctx context.Context, symbols []string) error {
	drop := make([]string, 0, len(symbols))
	h.mu.Lock()
	for _, raw := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(raw))
		if h.refcount[sym] == 0 {
			continue
		}
		h.refcount[sym]--
		if h.refcount[sym] == 0 {
			delete(h.refcount, sym)
			delete(h.idBySymbol, sym)
			delete(h.lastPublish, sym)
			drop = append(drop, sym)
		}
	}
	h.mu.Unlock()
	if h.stream != nil && len(drop) > 0 {
		h.stream.Unsubscribe(ctx, drop...)
	}
	return nil
}

// onTrade is the upstream tick callback: tap the observer (every tick), then throttle + publish
// a quote to the outbox.
func (h *marketDataHub) onTrade(t alpaca.Trade) {
	sym := strings.ToUpper(t.Symbol)
	// Alert engine tap — BEFORE the throttle (it needs every tick for slope + crossings) and
	// non-blocking (this is the WS read goroutine; a stall backs up the socket).
	if obs, ok := h.tickObserver.Load().(TickObserver); ok && obs != nil {
		obs(sym, t.Price, t.Time)
	}
	h.mu.Lock()
	id, known := h.idBySymbol[sym]
	last := h.lastPublish[sym]
	now := time.Now()
	if !known || now.Sub(last) < quoteThrottle {
		h.mu.Unlock()
		return
	}
	h.lastPublish[sym] = now
	h.mu.Unlock()

	ts := t.Time
	if ts.IsZero() {
		ts = now
	}
	if err := h.quotes.Publish(context.Background(), Quote{
		Symbol:       sym,
		InstrumentID: id,
		Last:         t.Price,
		Ts:           ts.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		slog.Warn("market: tick publish failed", "component", "market", "symbol", sym, "err", err)
	}
}
