package service

import (
	"math"
	"sort"
	"time"
)

// T1 / L0 — adjust-on-read (TRADING_PRD §5).
//
// Bars are stored RAW. Adjusted prices mutate retroactively on every split and dividend, so
// persisting them would make stored history silently change under you and backtests stop being
// reproducible. Instead the corporate-actions log is replayed over raw bars at read time — which
// keeps the result deterministic and makes THIS logic unit-testable, the whole point of the choice.

// CorporateActionType discriminates the log entries.
const (
	ActionSplit        = "split"
	ActionCashDividend = "cash_dividend"
)

// CorporateAction is one split or cash dividend on an instrument.
//
// ExDate is the first session that trades WITHOUT the entitlement, so bars STRICTLY BEFORE it are
// the ones needing adjustment; the ex-date bar itself is already ex and is left alone.
type CorporateAction struct {
	Type   string    `json:"type"`
	ExDate time.Time `json:"exDate"`
	// SplitRatio is NEW shares per OLD share (4-for-1 → 4.0; 1-for-10 reverse → 0.1). Splits only.
	SplitRatio float64 `json:"splitRatio,omitempty"`
	// CashAmount is cash per share. Dividends only.
	CashAmount float64 `json:"cashAmount,omitempty"`
}

// AdjustMode selects which actions are replayed.
type AdjustMode string

const (
	// AdjustNone returns bars exactly as stored.
	AdjustNone AdjustMode = "none"
	// AdjustSplits replays splits only — the price-continuity fix. Without it a 4-for-1 split
	// reads as a -75% crash, which would corrupt any return/indicator computed across it.
	AdjustSplits AdjustMode = "splits"
	// AdjustTotal replays splits AND cash dividends (total-return). What a backtest holding
	// through a dividend needs; NOT what a price chart should show, since the resulting historical
	// prices are below what actually traded.
	AdjustTotal AdjustMode = "total"
)

// ParseAdjustMode maps a query value to a mode, defaulting to split-adjusted.
func ParseAdjustMode(s string) AdjustMode {
	switch AdjustMode(s) {
	case AdjustNone:
		return AdjustNone
	case AdjustTotal:
		return AdjustTotal
	default:
		return AdjustSplits
	}
}

// AdjustBars replays actions over raw bars and returns the adjusted series. Pure: it does not
// mutate its inputs. `bars` must be ascending by time (the repo's contract); `actions` may be in
// any order.
//
// A bar is scaled by the CUMULATIVE factor of every action dated after it:
//   - split ratio R → prices ÷ R, volume × R (same dollar volume, more shares)
//   - cash dividend D → prices × (C-D)/C, where C is the close of the last session before the
//     ex-date (the standard pre-ex reference); volume unchanged
func AdjustBars(bars []Bar, actions []CorporateAction, mode AdjustMode) []Bar {
	if mode == AdjustNone || len(bars) == 0 || len(actions) == 0 {
		return bars
	}

	// Keep only the actions this mode replays, then order by ex-date so the walk below can consume
	// them newest-first in one pass.
	kept := make([]CorporateAction, 0, len(actions))
	for _, a := range actions {
		switch a.Type {
		case ActionSplit:
			if a.SplitRatio > 0 && a.SplitRatio != 1 {
				kept = append(kept, a)
			}
		case ActionCashDividend:
			if mode == AdjustTotal && a.CashAmount > 0 {
				kept = append(kept, a)
			}
		}
	}
	if len(kept) == 0 {
		return bars
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].ExDate.Before(kept[j].ExDate) })

	// Precompute each action's factors against the RAW series (a dividend needs the pre-ex close,
	// which must be read before anything is rescaled).
	type factor struct {
		exDate time.Time
		price  float64
		volume float64
	}
	factors := make([]factor, 0, len(kept))
	for _, a := range kept {
		switch a.Type {
		case ActionSplit:
			factors = append(factors, factor{exDate: a.ExDate, price: 1 / a.SplitRatio, volume: a.SplitRatio})
		case ActionCashDividend:
			c := preExClose(bars, a.ExDate)
			// No prior bar, or a dividend that swallows the price: not adjustable — skip rather
			// than emit a zero/negative factor that would poison the whole series.
			if c <= 0 || a.CashAmount >= c {
				continue
			}
			factors = append(factors, factor{exDate: a.ExDate, price: (c - a.CashAmount) / c, volume: 1})
		}
	}
	if len(factors) == 0 {
		return bars
	}

	// Walk newest → oldest accumulating factors: an action applies to every bar before its ex-date,
	// so once consumed it stays in the running product for all older bars.
	out := make([]Bar, len(bars))
	copy(out, bars)
	cumPrice, cumVol := 1.0, 1.0
	fi := len(factors) - 1
	for i := len(out) - 1; i >= 0; i-- {
		for fi >= 0 && factors[fi].exDate.After(out[i].Time) {
			cumPrice *= factors[fi].price
			cumVol *= factors[fi].volume
			fi--
		}
		if cumPrice == 1 && cumVol == 1 {
			continue
		}
		out[i].Open *= cumPrice
		out[i].High *= cumPrice
		out[i].Low *= cumPrice
		out[i].Close *= cumPrice
		out[i].Volume = int64(math.Round(float64(out[i].Volume) * cumVol))
	}
	return out
}

// preExClose returns the close of the last bar strictly before exDate (0 if there is none).
// bars are ascending, so a binary search finds the boundary.
func preExClose(bars []Bar, exDate time.Time) float64 {
	i := sort.Search(len(bars), func(i int) bool { return !bars[i].Time.Before(exDate) })
	if i == 0 {
		return 0
	}
	return bars[i-1].Close
}
