package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

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

// --- slope + hysteresis tuning (shared by the service seed + the engine eval) ---------------
// These are the ONE source of truth for the algorithm's constants so eval and tests never drift.
const (
	// emaAlpha smooths the tick-to-tick delta into a signed velocity proxy. Higher = more
	// responsive, lower = more noise-damping. 0.3 dampens isolated prints while tracking a
	// sustained move within 2–3 ticks.
	emaAlpha = 0.3
	// bandPct / bandFloor: hysteresis band H = max(bandPct*threshold, bandFloor). % -relative so
	// a $5 and a $500 stock behave alike; the floor prevents a degenerate zero band.
	bandPct   = 0.001 // 0.1%
	bandFloor = 0.01
	// epsPct / epsFloor: slope floor ε = max(epsPct*threshold, epsFloor). A real move clears it;
	// a single dampened print stays under it.
	epsPct   = 0.0002 // 0.02%
	epsFloor = 0.005
)

// AlertBand returns the hysteresis band H for a threshold.
func AlertBand(threshold float64) float64 { return math.Max(bandPct*math.Abs(threshold), bandFloor) }

// AlertEps returns the slope floor ε for a threshold.
func AlertEps(threshold float64) float64 { return math.Max(epsPct*math.Abs(threshold), epsFloor) }

// EvalAlert is the pure fire/re-arm decision — no clock, no DB, no I/O. THE correctness core.
// prevPrice is the price before this tick; slope is the EMA velocity proxy. Returns whether the
// alert fires now and its new armed state.
func EvalAlert(direction string, prevPrice, price, slope, threshold, band, eps float64, armed bool) (fire, newArmed bool) {
	switch direction {
	case AlertAbove:
		if armed && prevPrice < threshold && price >= threshold && slope >= eps {
			return true, false // genuine up-cross + confirming upward velocity
		}
		if !armed && price <= threshold-band && slope <= -eps {
			return false, true // pulled back a full band below with downward velocity → re-arm
		}
	case AlertBelow:
		if armed && prevPrice > threshold && price <= threshold && slope <= -eps {
			return true, false
		}
		if !armed && price >= threshold+band && slope >= eps {
			return false, true
		}
	}
	return false, armed
}

// UpdateSlope folds one tick delta into the EMA velocity proxy.
func UpdateSlope(prevSlope, prevPrice, price float64) float64 {
	return emaAlpha*(price-prevPrice) + (1-emaAlpha)*prevSlope
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
	instruments InstrumentResolver
	snapshots   QuoteSnapshotSource // optional — seeds armed from the last-known price
}

func NewAlertService(repo AlertRepository, instruments InstrumentResolver, snapshots QuoteSnapshotSource) AlertService {
	return &defaultAlertService{repo: repo, instruments: instruments, snapshots: snapshots}
}

func (s *defaultAlertService) Create(ctx context.Context, userID, symbol, direction string, threshold float64) (CreateAlertResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return CreateAlertResult{}, ErrUnauthenticated
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return CreateAlertResult{}, BadRequest("symbol is required")
	}
	if direction != AlertAbove && direction != AlertBelow {
		return CreateAlertResult{}, BadRequest(`direction must be "above" or "below"`)
	}
	if threshold <= 0 {
		return CreateAlertResult{}, BadRequest("threshold must be positive")
	}
	id, ok, err := s.instruments.ResolveInstrumentID(ctx, sym)
	if err != nil {
		return CreateAlertResult{}, err
	}
	if !ok {
		return CreateAlertResult{}, BadRequest("unknown symbol " + sym)
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
		return nil, ErrUnauthenticated
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
		return ErrUnauthenticated
	}
	if strings.TrimSpace(id) == "" {
		return BadRequest("alert id is required")
	}
	return s.repo.Delete(ctx, userID, id)
}

func (s *defaultAlertService) Pause(ctx context.Context, userID, id string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUnauthenticated
	}
	if strings.TrimSpace(id) == "" {
		return BadRequest("alert id is required")
	}
	return s.repo.SetStatus(ctx, userID, id, "paused")
}
