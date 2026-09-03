package turnstate

import "testing"

func TestTurnStateTransitions(t *testing.T) {
	machine := New()
	for _, event := range []Event{StreamStarted, ActionProposed, ActionResolved, ProviderDone} {
		if err := machine.Transition(event); err != nil {
			t.Fatalf("transition %s: %v", event, err)
		}
	}
	if machine.State() != Completed || !machine.Terminal() {
		t.Fatalf("state = %s", machine.State())
	}
	if err := machine.Transition(StreamStarted); err == nil {
		t.Fatal("terminal state accepted another event")
	}
}

func TestTurnStateTerminalOutcomes(t *testing.T) {
	for _, tc := range []struct {
		event Event
		want  State
	}{{Cancel, Cancelled}, {Fail, Failed}} {
		machine := New()
		if err := machine.Transition(tc.event); err != nil {
			t.Fatal(err)
		}
		if machine.State() != tc.want || !machine.Terminal() {
			t.Fatalf("%s => %s, want %s", tc.event, machine.State(), tc.want)
		}
	}
}
