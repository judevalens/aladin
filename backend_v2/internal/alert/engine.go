package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"aladin/backend_v2/internal/safego"
	coreservice "aladin/backend_v2/internal/service"
)

// AlertEngine evaluates price alerts on live ticks. It is a SINGLETON (one Alpaca WS) and runs
// in the api process next to the market hub. Efficiency contract: the tick path does zero
// DB/network and never blocks the WS read goroutine — OnTick is a non-blocking hand-off to a
// single eval goroutine that owns all mutable state (the alert index + per-symbol slope). Cross-
// process discovery (copilot creates alerts in the MCP process) + missed-tick/off-hours recovery
// come from a periodic reconcile that reloads the DB, drives market-hub demand, and snapshot-
// backstops. Delivery is via the transactional outbox (AlertRepository.Fire) — the engine never
// touches realtime, which is what keeps it relocatable.
type AlertEngine struct {
	repo      AlertRepository
	demand    AlertDemand
	snapshots coreservice.QuoteSnapshotSource
	now       func() time.Time

	reconcileEvery time.Duration
	tickCh         chan tickEvent
	cmdCh          chan reconcileCmd

	// eval-goroutine-owned (no locks — single owner):
	index    map[string][]*engineAlert // symbol → its active alerts
	symState map[string]*symState

	// reconcile-goroutine-owned:
	engineDemand map[string]bool // symbols the engine currently holds an upstream sub for

	dropped atomic.Uint64 // ticks shed by drop-on-full (observability)
}

// AlertDemand is the narrow slice of MarketDataService the engine needs to make alert symbols
// stream even when no UI is watching them.
type AlertDemand interface {
	Subscribe(ctx context.Context, symbols []string) error
	Unsubscribe(ctx context.Context, symbols []string) error
}

type tickEvent struct {
	symbol string
	price  float64
	at     time.Time
}

type reconcileCmd struct {
	alerts     []Alert            // the full active set (source of truth)
	seedPrices map[string]float64 // snapshot price per symbol, for slope seeding + backstop
}

type engineAlert struct {
	id, userID, instrumentID, symbol, direction string
	threshold, band, eps                        float64
	armed                                       bool
}

type symState struct {
	window    *PriceWindow // rolling window → real velocity (Δprice/Δtime)
	lastPrice float64      // price before the current tick, for the crossing edge
}

const (
	defaultReconcileEvery = 20 * time.Second
	tickChBuffer          = 4096
)

// NewAlertEngine builds the engine. demand is the market hub (satisfies AlertDemand); snapshots
// seeds slope + backs up missed ticks (nil-safe — without it, reconcile can't backstop).
func NewAlertEngine(repo AlertRepository, demand AlertDemand, snapshots coreservice.QuoteSnapshotSource) *AlertEngine {
	return &AlertEngine{
		repo:           repo,
		demand:         demand,
		snapshots:      snapshots,
		now:            time.Now,
		reconcileEvery: defaultReconcileEvery,
		tickCh:         make(chan tickEvent, tickChBuffer),
		cmdCh:          make(chan reconcileCmd, 1),
		index:          map[string][]*engineAlert{},
		symState:       map[string]*symState{},
		engineDemand:   map[string]bool{},
	}
}

// OnTick is the market-hub observer — called on the WS read goroutine. It MUST NOT block:
// a non-blocking send; when the buffer is full the tick is dropped (the reconcile backstop
// covers it). Symbols with no alert never reach here in a blocking way — the send is O(1).
func (e *AlertEngine) OnTick(symbol string, price float64, at time.Time) {
	select {
	case e.tickCh <- tickEvent{symbol: strings.ToUpper(symbol), price: price, at: at}:
	default:
		e.dropped.Add(1)
	}
}

// Start launches the eval goroutine + the reconcile ticker; both stop on ctx cancel.
// Supervised: a panic in either loop is recovered and the loop restarted (a nil-deref in
// eval must not silently stop all alerts from ever firing again).
func (e *AlertEngine) Start(ctx context.Context) {
	safego.Loop(ctx, "alert.eval", e.evalLoop)
	safego.Loop(ctx, "alert.reconcile", e.reconcileLoop)
}

// evalLoop is the SOLE owner of index + symState. It drains ticks (update slope + evaluate)
// and applies reconcile commands. No locks — everything mutable flows through here.
func (e *AlertEngine) evalLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-e.cmdCh:
			e.applyReconcile(ctx, cmd)
		case ev := <-e.tickCh:
			e.onTickEval(ctx, ev)
		}
	}
}

func (e *AlertEngine) onTickEval(ctx context.Context, ev tickEvent) {
	alerts := e.index[ev.symbol]
	if len(alerts) == 0 {
		return
	}
	st := e.symState[ev.symbol]
	if st == nil {
		st = &symState{window: NewPriceWindow(VelWindow), lastPrice: ev.price}
		st.window.Push(ev.at, ev.price)
		e.symState[ev.symbol] = st
		return // first tick just seeds the baseline — no crossing yet
	}
	prev := st.lastPrice
	st.window.Push(ev.at, ev.price)
	vel := st.window.Velocity()
	st.lastPrice = ev.price
	for _, a := range alerts {
		fire, newArmed := EvalAlert(a.direction, prev, ev.price, vel, a.threshold, a.band, a.eps, a.armed)
		if fire {
			e.fire(ctx, a, ev.price, ev.at)
			a.armed = false
		} else {
			a.armed = newArmed
		}
	}
}

// applyReconcile rebuilds the alert index from the DB truth, seeds new symbols' slope state
// from snapshots, and snapshot-backstops (fires alerts already past their threshold that were
// missed while ticks were dropped or off-hours). Runs in the eval goroutine (single owner).
func (e *AlertEngine) applyReconcile(ctx context.Context, cmd reconcileCmd) {
	// Normalize seed prices to UPPERCASE keys so every downstream lookup (seed + backstop)
	// aligns with the uppercase index + OnTick.
	seed := make(map[string]float64, len(cmd.seedPrices))
	for sym, price := range cmd.seedPrices {
		seed[strings.ToUpper(sym)] = price
	}
	cmd.seedPrices = seed

	// Index by UPPERCASE symbol to match OnTick's normalization (alert symbols are already
	// uppercase in production, but normalize defensively so a stray case never silently misses).
	next := map[string][]*engineAlert{}
	for _, a := range cmd.alerts {
		sym := strings.ToUpper(a.Symbol)
		ea := &engineAlert{
			id: a.ID, userID: a.UserID, instrumentID: a.InstrumentID, symbol: sym,
			direction: a.Direction, threshold: a.Threshold,
			band: AlertBand(a.Threshold), eps: AlertVelEps(a.Threshold), armed: a.Armed,
		}
		next[sym] = append(next[sym], ea)
	}
	e.index = next

	// Seed baseline state for any newly-referenced symbol from its snapshot price. The window
	// starts with this one sample (velocity 0) and fills in as real ticks arrive.
	for sym, price := range cmd.seedPrices {
		up := strings.ToUpper(sym)
		if _, ok := e.symState[up]; !ok {
			st := &symState{window: NewPriceWindow(VelWindow), lastPrice: price}
			st.window.Push(e.now(), price)
			e.symState[up] = st
		}
	}
	// Drop state for symbols no longer referenced.
	for sym := range e.symState {
		if _, ok := next[sym]; !ok {
			delete(e.symState, sym)
		}
	}

	// Backstop: fire any armed alert already satisfied at the snapshot price — it crossed while
	// we weren't watching. Fire disarms (DB + memory), so this can't double-fire with the tick path.
	for sym, alerts := range next {
		price, ok := cmd.seedPrices[sym]
		if !ok || price <= 0 {
			continue
		}
		for _, a := range alerts {
			if a.armed && satisfied(a.direction, price, a.threshold) {
				e.fire(ctx, a, price, e.now())
				a.armed = false
			}
		}
	}
}

func satisfied(direction string, price, threshold float64) bool {
	switch direction {
	case AlertAbove:
		return price >= threshold
	case AlertBelow:
		return price <= threshold
	}
	return false
}

// fire persists the trigger + notification + outbox event (one tx) and disarms. Best-effort:
// a DB error is logged, not retried (the reconcile backstop re-attempts on the next sweep if
// the alert is still armed — MarkFired's armed=true guard makes that safe).
func (e *AlertEngine) fire(ctx context.Context, a *engineAlert, price float64, at time.Time) {
	data, _ := json.Marshal(map[string]any{
		"alertId": a.id, "instrumentId": a.instrumentID, "symbol": a.symbol,
		"direction": a.direction, "threshold": a.threshold, "price": price,
	})
	title := fmt.Sprintf("%s crossed %s %.2f", a.symbol, a.direction, a.threshold)
	body := fmt.Sprintf("Last trade %.2f", price)
	n := Notification{UserID: a.userID, Kind: "price_alert", Title: title, Body: body, Data: data}
	if err := e.repo.Fire(ctx, a.id, price, at, n); err != nil {
		slog.Warn("alert: fire failed", "component", "alerts", "alert", a.id, "symbol", a.symbol, "err", err)
		return
	}
	slog.Info("alert: fired", "component", "alerts", "symbol", a.symbol, "direction", a.direction, "threshold", a.threshold, "price", price)
}

// reconcileLoop periodically reloads active alerts, drives market-hub demand (so alert symbols
// stream even without UI), fetches seed snapshots, and hands the eval goroutine a reconcile
// command. This is also how MCP-created alerts are discovered. Runs an immediate first pass.
func (e *AlertEngine) reconcileLoop(ctx context.Context) {
	e.reconcileOnce(ctx)
	ticker := time.NewTicker(e.reconcileEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.reconcileOnce(ctx)
		}
	}
}

func (e *AlertEngine) reconcileOnce(ctx context.Context) {
	alerts, err := e.repo.ListActive(ctx)
	if err != nil {
		slog.Warn("alert: reconcile load failed", "component", "alerts", "err", err)
		return
	}
	// Distinct symbols with active alerts → drive demand + seed prices.
	symbols := map[string]bool{}
	for _, a := range alerts {
		symbols[a.Symbol] = true
	}
	e.updateDemand(ctx, symbols)

	seed := map[string]float64{}
	if e.snapshots != nil {
		for sym := range symbols {
			if q, ok, serr := e.snapshots.FetchSnapshot(ctx, sym); serr == nil && ok && q.Last > 0 {
				seed[sym] = q.Last
			}
		}
	}
	select {
	case e.cmdCh <- reconcileCmd{alerts: alerts, seedPrices: seed}:
	case <-ctx.Done():
	}
}

// updateDemand adds an engine upstream sub for each newly-alerted symbol and drops it when no
// alert references the symbol. The hub refcounts, so this never unsubscribes a UI-watched symbol.
func (e *AlertEngine) updateDemand(ctx context.Context, want map[string]bool) {
	if e.demand == nil {
		return
	}
	var add, drop []string
	for sym := range want {
		if !e.engineDemand[sym] {
			e.engineDemand[sym] = true
			add = append(add, sym)
		}
	}
	for sym := range e.engineDemand {
		if !want[sym] {
			delete(e.engineDemand, sym)
			drop = append(drop, sym)
		}
	}
	if len(add) > 0 {
		if err := e.demand.Subscribe(ctx, add); err != nil {
			// Roll back the demand marks so the NEXT reconcile retries these symbols. Otherwise a
			// failed subscribe leaves them recorded as satisfied forever, and the alert silently
			// fires only via the 20s snapshot backstop — never on live-tick crossings.
			slog.Warn("alert engine: demand subscribe failed; rolling back to retry next reconcile",
				"component", "alerts", "symbols", add, "err", err)
			for _, sym := range add {
				delete(e.engineDemand, sym)
			}
		}
	}
	if len(drop) > 0 {
		if err := e.demand.Unsubscribe(ctx, drop); err != nil {
			slog.Warn("alert engine: demand unsubscribe failed", "component", "alerts", "symbols", drop, "err", err)
		}
	}
}
