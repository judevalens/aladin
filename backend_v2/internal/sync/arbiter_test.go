package sync

import (
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
)

func TestChooseCycleCreatesRefreshWhenDueAndNoCycles(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	source := &db.Source{
		ID:           "source-1",
		SyncInterval: 300,
	}

	decision := ChooseCycle(source, nil, now)
	if decision.Action != DecisionCreateRefresh {
		t.Fatalf("action = %q, want %q", decision.Action, DecisionCreateRefresh)
	}
}

func TestChooseCyclePrefersOldestActiveCycleWhenRefreshNotDue(t *testing.T) {
	t.Parallel()

	lastRefresh := time.Date(2026, 4, 5, 11, 59, 0, 0, time.UTC)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	source := &db.Source{
		ID:            "source-1",
		SyncInterval:  300,
		LastRefreshAt: &lastRefresh,
	}

	older := &db.SyncCycle{
		ID:        "cycle-old",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
	}
	newer := &db.SyncCycle{
		ID:        "cycle-new",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: time.Date(2026, 4, 5, 11, 30, 0, 0, time.UTC),
	}

	decision := ChooseCycle(source, []*db.SyncCycle{newer, older}, now)
	if decision.Action != DecisionRunCycle {
		t.Fatalf("action = %q, want %q", decision.Action, DecisionRunCycle)
	}
	if decision.Cycle == nil || decision.Cycle.ID != "cycle-new" {
		t.Fatalf("cycle = %#v, want cycle-new", decision.Cycle)
	}
}

func TestChooseCycleSkipsWhenSourceAlreadyRunning(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	source := &db.Source{
		ID:           "source-1",
		SyncInterval: 300,
	}
	running := &db.SyncCycle{
		ID:        "cycle-1",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusRunning,
		CreatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
	}

	decision := ChooseCycle(source, []*db.SyncCycle{running}, now)
	if decision.Action != DecisionSkip {
		t.Fatalf("action = %q, want %q", decision.Action, DecisionSkip)
	}
	if decision.Reason != "cycle_running" {
		t.Fatalf("reason = %q, want cycle_running", decision.Reason)
	}
}

func TestChooseCycleCreatesRefreshWhenDueAlongsideOlderActiveCycle(t *testing.T) {
	t.Parallel()

	lastRefresh := time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	source := &db.Source{
		ID:            "source-1",
		SyncInterval:  300,
		LastRefreshAt: &lastRefresh,
	}
	older := &db.SyncCycle{
		ID:        "cycle-old",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: time.Date(2026, 4, 5, 11, 1, 0, 0, time.UTC),
	}

	decision := ChooseCycle(source, []*db.SyncCycle{older}, now)
	if decision.Action != DecisionCreateRefresh {
		t.Fatalf("action = %q, want %q", decision.Action, DecisionCreateRefresh)
	}
}

func TestChooseCycleContinuesNewestActiveCycleWhenRefreshNotDue(t *testing.T) {
	t.Parallel()

	lastRefresh := time.Date(2026, 4, 5, 11, 58, 0, 0, time.UTC)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	source := &db.Source{
		ID:            "source-1",
		SyncInterval:  300,
		LastRefreshAt: &lastRefresh,
	}
	older := &db.SyncCycle{
		ID:        "cycle-old",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
	}
	newer := &db.SyncCycle{
		ID:        "cycle-new",
		Kind:      CycleKindRefresh,
		Status:    CycleStatusActive,
		CreatedAt: time.Date(2026, 4, 5, 11, 30, 0, 0, time.UTC),
	}

	decision := ChooseCycle(source, []*db.SyncCycle{newer, older}, now)
	if decision.Action != DecisionRunCycle {
		t.Fatalf("action = %q, want %q", decision.Action, DecisionRunCycle)
	}
	if decision.Cycle == nil || decision.Cycle.ID != "cycle-new" {
		t.Fatalf("cycle = %#v, want cycle-new", decision.Cycle)
	}
}
