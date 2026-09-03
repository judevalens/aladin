// Package turnstate owns the legal lifecycle of one Copilot provider turn.
package turnstate

import "fmt"

type State string
type Event string

const (
	Preparing        State = "preparing"
	Streaming        State = "streaming"
	AwaitingApproval State = "awaiting_approval"
	Completed        State = "completed"
	Cancelled        State = "cancelled"
	Failed           State = "failed"

	StreamStarted  Event = "stream_started"
	ActionProposed Event = "action_proposed"
	ActionResolved Event = "action_resolved"
	ProviderDone   Event = "provider_done"
	Cancel         Event = "cancel"
	Fail           Event = "fail"
)

type Machine struct{ state State }

func New() *Machine             { return &Machine{state: Preparing} }
func (m *Machine) State() State { return m.state }

func (m *Machine) Transition(event Event) error {
	next, ok := transitions[m.state][event]
	if !ok {
		return fmt.Errorf("illegal copilot turn transition %s --%s--> ?", m.state, event)
	}
	m.state = next
	return nil
}

func (m *Machine) Terminal() bool {
	return m.state == Completed || m.state == Cancelled || m.state == Failed
}

var transitions = map[State]map[Event]State{
	Preparing:        {StreamStarted: Streaming, Cancel: Cancelled, Fail: Failed},
	Streaming:        {ActionProposed: AwaitingApproval, ActionResolved: Streaming, ProviderDone: Completed, Cancel: Cancelled, Fail: Failed},
	AwaitingApproval: {ActionProposed: AwaitingApproval, ActionResolved: Streaming, ProviderDone: Completed, Cancel: Cancelled, Fail: Failed},
}
