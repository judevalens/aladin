package service

import "testing"

// feedTicks runs a price series through the slope EMA + EvalAlert exactly as the engine does,
// returning how many times the alert fired. This is the correctness harness for the
// slope/hysteresis re-arm behavior.
func feedTicks(direction string, threshold float64, prices []float64, startArmed bool) int {
	band := AlertBand(threshold)
	eps := AlertEps(threshold)
	armed := startArmed
	fires := 0
	var slope float64
	prev := prices[0] // seed: no fire on the very first tick
	for _, p := range prices[1:] {
		slope = UpdateSlope(slope, prev, p)
		fire, newArmed := EvalAlert(direction, prev, p, slope, threshold, band, eps, armed)
		if fire {
			fires++
		}
		armed = newArmed
		prev = p
	}
	return fires
}

func TestEvalAlert_FiresOnceThenQuietOnJitter(t *testing.T) {
	// Genuine rise through 100, then jitter around it. Must fire exactly once.
	prices := []float64{98, 98.5, 99.2, 99.8, 100.3, 100.1, 99.9, 100.2, 99.95, 100.05, 99.97}
	if got := feedTicks(AlertAbove, 100, prices, true); got != 1 {
		t.Fatalf("jitter fires = %d, want 1", got)
	}
}

func TestEvalAlert_FiresTwiceOnCrossPullbackRecross(t *testing.T) {
	// Rise through 100 → deep pullback well below (past band, downward slope) → rise back through.
	prices := []float64{
		98, 98.6, 99.3, 100.4, 101, // cross up → FIRE 1, disarm
		100, 99, 98, 97, 96, // deep pullback with negative slope → re-arm
		97, 98.5, 99.4, 100.5, 101, // cross up again → FIRE 2
	}
	if got := feedTicks(AlertAbove, 100, prices, true); got != 2 {
		t.Fatalf("cross/pullback/recross fires = %d, want 2", got)
	}
}

func TestEvalAlert_GenuinePrintAboveFires(t *testing.T) {
	// A real trade print above the level IS a legitimate cross (Alpaca sends real consolidated
	// prints, not noisy quotes) — it should fire exactly once, then the disarm keeps it quiet.
	prices := []float64{90, 90, 100.5, 90, 90}
	if got := feedTicks(AlertAbove, 100, prices, true); got != 1 {
		t.Fatalf("genuine print above fires = %d, want 1", got)
	}
}

func TestEvalAlert_ShallowDipDoesNotReArm(t *testing.T) {
	// After a fire, a single-tick dip to the band edge must NOT re-arm — the EMA-lagged slope
	// hasn't genuinely reversed. Only a sustained pullback flips it. This is the smart re-arm:
	// a real momentum reversal, not a dead-cat dip, is required.
	prices := []float64{
		97, 98, 99.5, 100.6, 101, // cross up → FIRE, disarm
		100.4, 99.9, 100.5, 101, // one dip toward the band then straight back up — must NOT re-fire
	}
	if got := feedTicks(AlertAbove, 100, prices, true); got != 1 {
		t.Fatalf("shallow dip re-armed and re-fired: fires = %d, want 1", got)
	}
}

func TestEvalAlert_BelowSymmetric(t *testing.T) {
	// Fall through 50 once amid jitter → exactly one fire.
	prices := []float64{53, 52, 51, 50.3, 49.6, 50.1, 49.9, 50.05, 49.95}
	if got := feedTicks(AlertBelow, 50, prices, true); got != 1 {
		t.Fatalf("below jitter fires = %d, want 1", got)
	}
}

func TestEvalAlert_ReArmNeedsFullBandAndSlope(t *testing.T) {
	threshold, band, eps := 100.0, AlertBand(100), AlertEps(100)
	// Disarmed, price dips just barely below threshold (not a full band) → must NOT re-arm.
	fire, armed := EvalAlert(AlertAbove, 100.05, 100.0-band/2, -1, threshold, band, eps, false)
	if fire || armed {
		t.Fatalf("shallow dip re-armed: fire=%v armed=%v", fire, armed)
	}
	// Full band below with clear negative slope → re-arm.
	_, armed = EvalAlert(AlertAbove, 100.0, 100.0-band*2, -1, threshold, band, eps, false)
	if !armed {
		t.Fatal("full-band pullback with negative slope should re-arm")
	}
}

func TestEvalAlert_DisarmedDoesNotFire(t *testing.T) {
	threshold, band, eps := 100.0, AlertBand(100), AlertEps(100)
	// A disarmed alert crossing up again must not fire (it must re-arm first).
	fire, _ := EvalAlert(AlertAbove, 99, 101, +5, threshold, band, eps, false)
	if fire {
		t.Fatal("disarmed alert should not fire on a cross")
	}
}

func TestInitialArmed(t *testing.T) {
	// "above 100": armed only when comfortably below the band.
	if InitialArmed(AlertAbove, 105, 100) {
		t.Fatal("above-alert seeded above threshold should be DISARMED")
	}
	if !InitialArmed(AlertAbove, 95, 100) {
		t.Fatal("above-alert seeded well below should be armed")
	}
	if InitialArmed(AlertBelow, 95, 100) {
		t.Fatal("below-alert seeded below threshold should be DISARMED")
	}
	if !InitialArmed(AlertBelow, 105, 100) {
		t.Fatal("below-alert seeded well above should be armed")
	}
}
