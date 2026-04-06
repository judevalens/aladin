package sync

import (
	"sort"
	"time"

	"aladin/backend_v2/internal/db"
)

const (
	CycleKindRefresh  = "refresh"
	CycleKindBackfill = "backfill"

	CycleStatusActive   = "active"
	CycleStatusRunning  = "running"
	CycleStatusComplete = "complete"
	CycleStatusClosed   = "closed"
)

type DecisionAction string

const (
	DecisionSkip          DecisionAction = "skip"
	DecisionCreateRefresh DecisionAction = "create_refresh"
	DecisionRunCycle      DecisionAction = "run_cycle"
)

type Decision struct {
	Action DecisionAction
	Cycle  *db.SyncCycle
	Reason string
}

// ChooseCycle decides what turn a source should get next.
// Policy:
// - one running cycle blocks new work for the source
// - if refresh is due and no active refresh exists, create a refresh cycle
// - otherwise continue the oldest active cycle first
func ChooseCycle(source *db.Source, cycles []*db.SyncCycle, now time.Time) Decision {
	if source == nil {
		return Decision{Action: DecisionSkip, Reason: "nil_source"}
	}

	active := filterActiveCycles(cycles)
	if hasRunningCycle(active) {
		return Decision{Action: DecisionSkip, Reason: "cycle_running"}
	}
	if refreshDue(source, now) && !hasActiveKind(active, CycleKindRefresh) {
		return Decision{Action: DecisionCreateRefresh, Reason: "refresh_due"}
	}
	if len(active) == 0 {
		return Decision{Action: DecisionSkip, Reason: "no_active_cycle"}
	}

	sort.SliceStable(active, func(i, j int) bool {
		return active[i].CreatedAt.Before(active[j].CreatedAt)
	})
	return Decision{Action: DecisionRunCycle, Cycle: active[0], Reason: "oldest_active_cycle"}
}

func refreshDue(source *db.Source, now time.Time) bool {
	if source.SyncInterval <= 0 {
		return true
	}
	if source.LastRefreshAt == nil {
		return true
	}
	return source.LastRefreshAt.Add(time.Duration(source.SyncInterval)*time.Second).Before(now) ||
		source.LastRefreshAt.Add(time.Duration(source.SyncInterval)*time.Second).Equal(now)
}

func filterActiveCycles(cycles []*db.SyncCycle) []*db.SyncCycle {
	active := make([]*db.SyncCycle, 0, len(cycles))
	for _, cycle := range cycles {
		if cycle == nil {
			continue
		}
		if cycle.Status == CycleStatusActive || cycle.Status == CycleStatusRunning {
			active = append(active, cycle)
		}
	}
	return active
}

func hasRunningCycle(cycles []*db.SyncCycle) bool {
	for _, cycle := range cycles {
		if cycle.Status == CycleStatusRunning {
			return true
		}
	}
	return false
}

func hasActiveKind(cycles []*db.SyncCycle, kind string) bool {
	for _, cycle := range cycles {
		if cycle.Kind == kind {
			return true
		}
	}
	return false
}
