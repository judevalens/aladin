package alert

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// Alert is a recurring, self-re-arming price threshold on an instrument. `Armed` is the
// slope/hysteresis gate: it flips false on fire and true only after a genuine pullback past
// the hysteresis band. Status stays "active" across fires (recurring).
type Alert struct {
	ID             string  `json:"id"`
	UserID         string  `json:"userId"`
	InstrumentID   string  `json:"instrumentId"`
	Symbol         string  `json:"symbol"`
	Direction      string  `json:"direction"` // above | below
	Threshold      float64 `json:"threshold"`
	Armed          bool    `json:"armed"`
	Status         string  `json:"status"` // active | paused
	LastFiredAt    string  `json:"lastFiredAt,omitempty"`
	LastFiredPrice float64 `json:"lastFiredPrice,omitempty"`
	CreatedAt      string  `json:"createdAt"`
}

const (
	AlertAbove = "above"
	AlertBelow = "below"
)

// --- velocity + hysteresis tuning (shared by the service seed + the engine eval) -------------
// These are the ONE source of truth for the algorithm's constants so eval and tests never drift.
const (
	// VelWindow is the rolling window over which velocity is measured. Long enough to smooth
	// tick noise + be robust to tick DENSITY (a burst of prints doesn't inflate it), short
	// enough to react intraday. Velocity = Δprice/Δtime across the window (real price/sec).
	VelWindow = 30 * time.Second
	// bandPct / bandFloor: hysteresis band H = max(bandPct*threshold, bandFloor). % -relative so
	// a $5 and a $500 stock behave alike; the floor prevents a degenerate zero band.
	bandPct   = 0.001 // 0.1%
	bandFloor = 0.01
	// velEpsPct / velEpsFloor: velocity floor ε = max(velEpsPct*threshold, velEpsFloor), in
	// price PER SECOND. A move of ≈velEpsPct of price per second (0.002%/s ≈ 0.06%/30s) clears
	// it; sub-threshold drift stays under it. Unit-clean and tick-density-independent.
	velEpsPct   = 0.00002 // fraction of price per second
	velEpsFloor = 0.001   // $/s floor
)

// AlertBand returns the hysteresis band H for a threshold.
func AlertBand(threshold float64) float64 { return math.Max(bandPct*math.Abs(threshold), bandFloor) }

// AlertVelEps returns the velocity floor ε (price/sec) for a threshold.
func AlertVelEps(threshold float64) float64 {
	return math.Max(velEpsPct*math.Abs(threshold), velEpsFloor)
}

// EvalAlert is the pure fire/re-arm decision — no clock, no DB, no I/O. THE correctness core.
// prevPrice is the price before this tick; velocity is Δprice/Δtime (price/sec) over the rolling
// window. eps is the velocity floor. Returns whether the alert fires now and its new armed state.
func EvalAlert(direction string, prevPrice, price, velocity, threshold, band, eps float64, armed bool) (fire, newArmed bool) {
	switch direction {
	case AlertAbove:
		if armed && prevPrice < threshold && price >= threshold && velocity >= eps {
			return true, false // genuine up-cross + confirming upward velocity
		}
		if !armed && price <= threshold-band && velocity <= -eps {
			return false, true // pulled back a full band below with downward velocity → re-arm
		}
	case AlertBelow:
		if armed && prevPrice > threshold && price <= threshold && velocity <= -eps {
			return true, false
		}
		if !armed && price >= threshold+band && velocity >= eps {
			return false, true
		}
	}
	return false, armed
}

// priceSample is one (time, price) point in a symbol's rolling velocity window.
type priceSample struct {
	t     time.Time
	price float64
}

// PriceWindow is a bounded rolling window of recent ticks, from which velocity (Δprice/Δtime)
// is derived. It is eval-goroutine-owned in the engine (no locks). Bounded by both time
// (VelWindow) and sample count (to cap memory under bursty symbols).
type PriceWindow struct {
	samples []priceSample
	dur     time.Duration
}

const maxWindowSamples = 1024

// NewPriceWindow builds an empty window of the given duration.
func NewPriceWindow(dur time.Duration) *PriceWindow { return &PriceWindow{dur: dur} }

// Push adds a tick and evicts anything older than the window (and caps the sample count).
func (w *PriceWindow) Push(t time.Time, price float64) {
	w.samples = append(w.samples, priceSample{t: t, price: price})
	cutoff := t.Add(-w.dur)
	i := 0
	for i < len(w.samples) && w.samples[i].t.Before(cutoff) {
		i++
	}
	if i > 0 {
		w.samples = append(w.samples[:0], w.samples[i:]...)
	}
	if over := len(w.samples) - maxWindowSamples; over > 0 {
		w.samples = append(w.samples[:0], w.samples[over:]...)
	}
}

// Velocity is the net price change per second across the window (0 with <2 samples or ~0 Δt).
// Robust to tick density: it measures movement over TIME, not per tick.
func (w *PriceWindow) Velocity() float64 {
	if len(w.samples) < 2 {
		return 0
	}
	oldest, newest := w.samples[0], w.samples[len(w.samples)-1]
	dt := newest.t.Sub(oldest.t).Seconds()
	if dt <= 0.0009 { // guard: sub-ms window (all ticks same instant) → undefined velocity
		return 0
	}
	return (newest.price - oldest.price) / dt
}

// Last returns the most recent price and whether the window has any sample.
func (w *PriceWindow) Last() (float64, bool) {
	if len(w.samples) == 0 {
		return 0, false
	}
	return w.samples[len(w.samples)-1].price, true
}

// InitialArmed decides a new/reloaded alert's armed state from a seed price: armed only when the
// price is strictly on the not-yet-triggered side by a full band (so it can't fire on tick #1).
func InitialArmed(direction string, seedPrice, threshold float64) bool {
	band := AlertBand(threshold)
	switch direction {
	case AlertAbove:
		return seedPrice <= threshold-band
	case AlertBelow:
		return seedPrice >= threshold+band
	}
	return false
}

// --- service ---------------------------------------------------------------------------------

// CreateAlertResult carries the created alert plus a non-fatal warning (e.g. the condition is
// already satisfied at creation, so it was created disarmed).
type CreateAlertResult struct {
	Alert   Alert  `json:"alert"`
	Warning string `json:"warning,omitempty"`
}

// AlertRepository persists alerts. Fire is transactional: it disarms the alert, inserts the
// notification, and appends the outbox event in ONE tx (all-or-nothing — no duplicate fires).
type AlertRepository interface {
	Insert(ctx context.Context, a Alert) error
	ListByUser(ctx context.Context, userID string) ([]Alert, error)
	ListActive(ctx context.Context) ([]Alert, error)
	Fire(ctx context.Context, alertID string, price float64, at time.Time, n Notification) error
	Delete(ctx context.Context, userID, id string) error
	SetStatus(ctx context.Context, userID, id, status string) error
}

type AlertService interface {
	Create(ctx context.Context, userID, symbol, direction string, threshold float64) (CreateAlertResult, error)
	List(ctx context.Context, userID string) ([]Alert, error)
	Delete(ctx context.Context, userID, id string) error
	Pause(ctx context.Context, userID, id string) error
}

type defaultAlertService struct {
	repo        AlertRepository
	instruments coreservice.InstrumentResolver
	snapshots   coreservice.QuoteSnapshotSource // optional — seeds armed from the last-known price
}

func NewAlertService(repo AlertRepository, instruments coreservice.InstrumentResolver, snapshots coreservice.QuoteSnapshotSource) AlertService {
	return &defaultAlertService{repo: repo, instruments: instruments, snapshots: snapshots}
}

func (s *defaultAlertService) Create(ctx context.Context, userID, symbol, direction string, threshold float64) (CreateAlertResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return CreateAlertResult{}, coreservice.ErrUnauthenticated
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return CreateAlertResult{}, coreservice.BadRequest("symbol is required")
	}
	if direction != AlertAbove && direction != AlertBelow {
		return CreateAlertResult{}, coreservice.BadRequest(`direction must be "above" or "below"`)
	}
	if threshold <= 0 {
		return CreateAlertResult{}, coreservice.BadRequest("threshold must be positive")
	}
	id, ok, err := s.instruments.ResolveInstrumentID(ctx, sym)
	if err != nil {
		return CreateAlertResult{}, err
	}
	if !ok {
		return CreateAlertResult{}, coreservice.BadRequest("unknown symbol " + sym)
	}

	// Seed the armed state from the last-known price so an already-satisfied alert is created
	// DISARMED (and never spuriously fires on the first tick) — with a warning back to the user.
	armed := true
	warning := ""
	if s.snapshots != nil {
		if q, ok, serr := s.snapshots.FetchSnapshot(ctx, sym); serr == nil && ok && q.Last > 0 {
			armed = InitialArmed(direction, q.Last, threshold)
			if !armed {
				warning = fmt.Sprintf("%s is already at %.2f (%s %.2f). The alert is set but will fire only after it pulls back and crosses again.",
					sym, q.Last, direction, threshold)
			}
		}
	}

	a := Alert{
		ID:           uuid.NewString(),
		UserID:       userID,
		InstrumentID: id,
		Symbol:       sym,
		Direction:    direction,
		Threshold:    threshold,
		Armed:        armed,
		Status:       "active",
	}
	if err := s.repo.Insert(ctx, a); err != nil {
		return CreateAlertResult{}, err
	}
	return CreateAlertResult{Alert: a, Warning: warning}, nil
}

func (s *defaultAlertService) List(ctx context.Context, userID string) ([]Alert, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, coreservice.ErrUnauthenticated
	}
	items, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Alert{}
	}
	return items, nil
}

func (s *defaultAlertService) Delete(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(userID) == "" {
		return coreservice.ErrUnauthenticated
	}
	if strings.TrimSpace(id) == "" {
		return coreservice.BadRequest("alert id is required")
	}
	return s.repo.Delete(ctx, userID, id)
}

func (s *defaultAlertService) Pause(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(userID) == "" {
		return coreservice.ErrUnauthenticated
	}
	if strings.TrimSpace(id) == "" {
		return coreservice.BadRequest("alert id is required")
	}
	return s.repo.SetStatus(ctx, userID, id, "paused")
}
