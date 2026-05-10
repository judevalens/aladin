package sync

import (
	"sort"
	"time"

	"aladin/backend_v2/internal/db"
)

const (
	CycleKindRefresh   = "refresh"
	CycleKindHydration = "hydration"

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

type Arbiter interface {
	Decide(stream *db.ProviderStream, cycles []*db.SyncCycle, now time.Time) Decision
}

type FreshnessFirstArbiter struct{}

func NewFreshnessFirstArbiter() Arbiter {
	return FreshnessFirstArbiter{}
}

// ChooseCycle is the compatibility wrapper for the current default arbitration policy.
func ChooseCycle(stream *db.ProviderStream, cycles []*db.SyncCycle, now time.Time) Decision {
	return FreshnessFirstArbiter{}.Decide(stream, cycles, now)
}

// Decide chooses what turn a provider stream should get next.
// Policy:
// - one running cycle blocks new work for the provider stream
// - if refresh is due and no active refresh exists for the current refresh window, create a refresh cycle
// - otherwise run due hydration before continuing refresh pagination
func (FreshnessFirstArbiter) Decide(stream *db.ProviderStream, cycles []*db.SyncCycle, now time.Time) Decision {
	if stream == nil {
		return Decision{Action: DecisionSkip, Reason: "nil_stream"}
	}

	active := filterActiveCycles(cycles)
	if hasRunningCycle(active) {
		return Decision{Action: DecisionSkip, Reason: "cycle_running"}
	}
	if refreshDue(stream, now) && shouldCreateRefresh(stream, active) {
		return Decision{Action: DecisionCreateRefresh, Reason: "refresh_due"}
	}
	due := filterDueCycles(active, now)
	if len(due) == 0 {
		return Decision{Action: DecisionSkip, Reason: "no_active_cycle"}
	}

	sort.SliceStable(due, func(i, j int) bool {
		if due[i].Kind != due[j].Kind {
			return due[i].Kind == CycleKindHydration
		}
		iPicked := due[i].LastPickedAt
		jPicked := due[j].LastPickedAt
		if iPicked == nil && jPicked != nil {
			return true
		}
		if iPicked != nil && jPicked == nil {
			return false
		}
		if iPicked != nil && jPicked != nil && !iPicked.Equal(*jPicked) {
			return iPicked.Before(*jPicked)
		}
		if due[i].Kind == CycleKindRefresh {
			return due[i].CreatedAt.After(due[j].CreatedAt)
		}
		return due[i].CreatedAt.Before(due[j].CreatedAt)
	})
	if due[0].Kind == CycleKindHydration {
		return Decision{Action: DecisionRunCycle, Cycle: due[0], Reason: "due_hydration_cycle"}
	}
	return Decision{Action: DecisionRunCycle, Cycle: due[0], Reason: "due_active_cycle"}
}

func refreshDue(stream *db.ProviderStream, now time.Time) bool {
	if stream.SyncInterval <= 0 {
		return true
	}
	if stream.LastRefreshAt == nil {
		return true
	}
	return stream.LastRefreshAt.Add(time.Duration(stream.SyncInterval)*time.Second).Before(now) ||
		stream.LastRefreshAt.Add(time.Duration(stream.SyncInterval)*time.Second).Equal(now)
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

func filterDueCycles(cycles []*db.SyncCycle, now time.Time) []*db.SyncCycle {
	due := make([]*db.SyncCycle, 0, len(cycles))
	for _, cycle := range cycles {
		if cycle == nil || cycle.Status == CycleStatusRunning {
			continue
		}
		if cycle.Kind == CycleKindHydration && !hydrationDue(cycle, now) {
			continue
		}
		due = append(due, cycle)
	}
	return due
}

func hydrationDue(cycle *db.SyncCycle, now time.Time) bool {
	if cycle == nil || cycle.Kind != CycleKindHydration {
		return true
	}
	if cycle.LastHydratedAt == nil {
		return true
	}
	intervalSeconds := intFromAny(cycle.Cursor["hydration_interval_seconds"])
	if intervalSeconds <= 0 {
		return true
	}
	return !cycle.LastHydratedAt.Add(time.Duration(intervalSeconds) * time.Second).After(now)
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		var parsed int
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				return 0
			}
			parsed = parsed*10 + int(ch-'0')
		}
		return parsed
	default:
		return 0
	}
}

func shouldCreateRefresh(stream *db.ProviderStream, cycles []*db.SyncCycle) bool {
	if len(cycles) == 0 {
		return true
	}
	if stream == nil || stream.LastRefreshAt == nil || stream.SyncInterval <= 0 {
		return !hasActiveKind(cycles, CycleKindRefresh)
	}

	windowStart := stream.LastRefreshAt.Add(time.Duration(stream.SyncInterval) * time.Second)
	for _, cycle := range cycles {
		if cycle == nil || cycle.Kind != CycleKindRefresh {
			continue
		}
		if !cycle.CreatedAt.Before(windowStart) {
			return false
		}
	}
	return true
}
