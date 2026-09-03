package market

import (
	"math"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2024, 6, d, 0, 0, 0, 0, time.UTC) }

// series builds ascending daily bars with a flat OHLC at the given closes.
func series(closes ...float64) []Bar {
	out := make([]Bar, 0, len(closes))
	for i, c := range closes {
		out = append(out, Bar{
			Time: day(i + 1), Open: c, High: c, Low: c, Close: c, Volume: 100,
		})
	}
	return out
}

func closeAt(t *testing.T, bars []Bar, i int) float64 {
	t.Helper()
	if i >= len(bars) {
		t.Fatalf("bar %d out of range (%d bars)", i, len(bars))
	}
	return bars[i].Close
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// A 4-for-1 split must make the series continuous: pre-split bars divide by 4 and their volume
// multiplies by 4. Without this a split reads as a -75% crash.
func TestAdjustBars_SplitMakesSeriesContinuous(t *testing.T) {
	// Days 1-2 at 400 (pre-split), days 3-4 at 100 (post-split). Ex-date = day 3.
	bars := series(400, 400, 100, 100)
	actions := []CorporateAction{{Type: ActionSplit, ExDate: day(3), SplitRatio: 4}}

	got := AdjustBars(bars, actions, AdjustSplits)

	if !approx(closeAt(t, got, 0), 100) || !approx(closeAt(t, got, 1), 100) {
		t.Fatalf("pre-split closes = %v, %v; want 100, 100 (÷4)", got[0].Close, got[1].Close)
	}
	if !approx(closeAt(t, got, 2), 100) || !approx(closeAt(t, got, 3), 100) {
		t.Fatalf("post-split closes must be untouched, got %v, %v", got[2].Close, got[3].Close)
	}
	if got[0].Volume != 400 {
		t.Fatalf("pre-split volume = %d, want 400 (×4)", got[0].Volume)
	}
	if got[3].Volume != 100 {
		t.Fatalf("post-split volume must be untouched, got %d", got[3].Volume)
	}
	// OHLC must all move together.
	if !approx(got[0].Open, 100) || !approx(got[0].High, 100) || !approx(got[0].Low, 100) {
		t.Fatalf("OHLC not adjusted uniformly: %+v", got[0])
	}
}

// The ex-date bar already trades ex — only STRICTLY earlier bars adjust.
func TestAdjustBars_ExDateBarIsNotAdjusted(t *testing.T) {
	bars := series(400, 100)
	actions := []CorporateAction{{Type: ActionSplit, ExDate: day(2), SplitRatio: 4}}

	got := AdjustBars(bars, actions, AdjustSplits)

	if !approx(closeAt(t, got, 0), 100) {
		t.Fatalf("pre-ex close = %v, want 100", got[0].Close)
	}
	if !approx(closeAt(t, got, 1), 100) {
		t.Fatalf("ex-date bar = %v, want 100 (unadjusted)", got[1].Close)
	}
}

// Sequential splits compound: a bar before both is divided by the product of the ratios.
func TestAdjustBars_MultipleSplitsCompound(t *testing.T) {
	bars := series(800, 400, 100) // 2-for-1 on day 2, then 4-for-1 on day 3
	actions := []CorporateAction{
		{Type: ActionSplit, ExDate: day(3), SplitRatio: 4},
		{Type: ActionSplit, ExDate: day(2), SplitRatio: 2}, // deliberately out of order
	}

	got := AdjustBars(bars, actions, AdjustSplits)

	if !approx(closeAt(t, got, 0), 100) { // 800 / (2*4)
		t.Fatalf("oldest close = %v, want 100 (÷8, compounded)", got[0].Close)
	}
	if !approx(closeAt(t, got, 1), 100) { // 400 / 4
		t.Fatalf("middle close = %v, want 100 (÷4)", got[1].Close)
	}
	if !approx(closeAt(t, got, 2), 100) {
		t.Fatalf("newest close = %v, want 100 (untouched)", got[2].Close)
	}
}

// A reverse split (1-for-10 → ratio 0.1) scales the other way.
func TestAdjustBars_ReverseSplit(t *testing.T) {
	bars := series(1, 10)
	actions := []CorporateAction{{Type: ActionSplit, ExDate: day(2), SplitRatio: 0.1}}

	got := AdjustBars(bars, actions, AdjustSplits)

	if !approx(closeAt(t, got, 0), 10) {
		t.Fatalf("pre-reverse-split close = %v, want 10 (×10)", got[0].Close)
	}
	if got[0].Volume != 10 { // 100 * 0.1
		t.Fatalf("pre-reverse-split volume = %d, want 10", got[0].Volume)
	}
}

// Dividends are replayed only in total-return mode, scaled by (C-D)/C off the pre-ex close.
func TestAdjustBars_DividendOnlyInTotalMode(t *testing.T) {
	bars := series(100, 99) // $1 dividend, ex on day 2
	actions := []CorporateAction{{Type: ActionCashDividend, ExDate: day(2), CashAmount: 1}}

	splits := AdjustBars(bars, actions, AdjustSplits)
	if !approx(closeAt(t, splits, 0), 100) {
		t.Fatalf("split mode must ignore dividends, got %v", splits[0].Close)
	}

	total := AdjustBars(bars, actions, AdjustTotal)
	want := 100 * (100 - 1) / 100.0 // 99
	if !approx(closeAt(t, total, 0), want) {
		t.Fatalf("total-return close = %v, want %v", total[0].Close, want)
	}
	if total[0].Volume != 100 {
		t.Fatalf("a dividend must not change volume, got %d", total[0].Volume)
	}
}

// Splits and dividends compound in total mode.
func TestAdjustBars_SplitAndDividendCompound(t *testing.T) {
	bars := series(100, 99, 33) // $1 div ex day 2, then 3-for-1 ex day 3
	actions := []CorporateAction{
		{Type: ActionCashDividend, ExDate: day(2), CashAmount: 1},
		{Type: ActionSplit, ExDate: day(3), SplitRatio: 3},
	}

	got := AdjustBars(bars, actions, AdjustTotal)

	want := 100 * 0.99 / 3 // dividend factor then split
	if !approx(closeAt(t, got, 0), want) {
		t.Fatalf("oldest close = %v, want %v (dividend × split)", got[0].Close, want)
	}
}

// Guards: nothing to do, or unusable inputs, must return the series untouched — never a zero or
// negative factor that would poison every earlier bar.
func TestAdjustBars_Guards(t *testing.T) {
	bars := series(100, 100)
	split := []CorporateAction{{Type: ActionSplit, ExDate: day(2), SplitRatio: 4}}

	if got := AdjustBars(bars, split, AdjustNone); !approx(closeAt(t, got, 0), 100) {
		t.Fatalf("AdjustNone must return raw bars, got %v", got[0].Close)
	}
	if got := AdjustBars(bars, nil, AdjustTotal); !approx(closeAt(t, got, 0), 100) {
		t.Fatalf("no actions must return raw bars, got %v", got[0].Close)
	}
	if got := AdjustBars(nil, split, AdjustSplits); len(got) != 0 {
		t.Fatalf("no bars must return empty, got %d", len(got))
	}
	// A dividend at/over the pre-ex close is not adjustable.
	swallow := []CorporateAction{{Type: ActionCashDividend, ExDate: day(2), CashAmount: 100}}
	if got := AdjustBars(bars, swallow, AdjustTotal); !approx(closeAt(t, got, 0), 100) {
		t.Fatalf("dividend >= pre-ex close must be skipped, got %v", got[0].Close)
	}
	// A dividend with no prior bar has no reference close.
	noPrior := []CorporateAction{{Type: ActionCashDividend, ExDate: day(1), CashAmount: 1}}
	if got := AdjustBars(bars, noPrior, AdjustTotal); !approx(closeAt(t, got, 0), 100) {
		t.Fatalf("dividend with no prior bar must be skipped, got %v", got[0].Close)
	}
	// An action newer than every bar applies to all of them; one older than every bar applies to none.
	older := []CorporateAction{{Type: ActionSplit, ExDate: day(1), SplitRatio: 4}}
	if got := AdjustBars(bars, older, AdjustSplits); !approx(closeAt(t, got, 0), 100) {
		t.Fatalf("action at/before the first bar must not adjust it, got %v", got[0].Close)
	}
}

// AdjustBars must not mutate the caller's slice — the raw series is the source of truth and is
// reused (e.g. the same cached bars served at different adjust modes).
func TestAdjustBars_DoesNotMutateInput(t *testing.T) {
	bars := series(400, 100)
	actions := []CorporateAction{{Type: ActionSplit, ExDate: day(2), SplitRatio: 4}}

	_ = AdjustBars(bars, actions, AdjustSplits)

	if !approx(bars[0].Close, 400) || bars[0].Volume != 100 {
		t.Fatalf("input mutated: %+v", bars[0])
	}
}

func TestParseAdjustMode(t *testing.T) {
	cases := map[string]AdjustMode{
		"none": AdjustNone, "total": AdjustTotal, "splits": AdjustSplits,
		"": AdjustSplits, "garbage": AdjustSplits, // default is split-adjusted
	}
	for in, want := range cases {
		if got := ParseAdjustMode(in); got != want {
			t.Fatalf("ParseAdjustMode(%q) = %q, want %q", in, got, want)
		}
	}
}
