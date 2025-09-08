package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/permit"
)

func TestEveryStateIsReachableAndWellFormed(t *testing.T) {
	states := permit.States()
	if len(states) != 13 {
		t.Fatalf("states = %v", states)
	}
	for i := 1; i < len(states); i++ {
		if states[i] < states[i-1] {
			t.Fatalf("states are not sorted: %v", states)
		}
	}
	for _, s := range states {
		if !permit.KnownState(s) {
			t.Errorf("%s is listed but not known", s)
		}
		if s == permit.StateDraft {
			continue
		}
		if _, ok := permit.Path(permit.StateDraft, s); !ok {
			t.Errorf("%s cannot be reached from a draft", s)
		}
	}
	if permit.KnownState(permit.State("pending")) {
		t.Error("there is no pending state")
	}
}

func TestTerminalStates(t *testing.T) {
	terminal := []permit.State{permit.StateClosed, permit.StateRejected, permit.StateWithdrawn}
	for _, s := range terminal {
		if !s.Terminal() || s.Live() {
			t.Errorf("%s is terminal", s)
		}
		if len(s.Next()) != 0 {
			t.Errorf("%s has successors %v", s, s.Next())
		}
	}
	for _, s := range []permit.State{permit.StateDraft, permit.StateEstimated, permit.StateInspected} {
		if s.Terminal() || !s.Live() {
			t.Errorf("%s is still live", s)
		}
	}
	if permit.State("pending").Live() {
		t.Error("an unknown state is not live")
	}
	if permit.State("pending").Terminal() {
		t.Error("nor terminal")
	}
	if permit.State("pending").Next() != nil {
		t.Error("nor does it have successors")
	}
}

func TestAllowedTransitions(t *testing.T) {
	if !permit.StateDraft.CanMoveTo(permit.StateSubmitted) {
		t.Error("a draft can be submitted")
	}
	if permit.StateDraft.CanMoveTo(permit.StateClosed) {
		t.Error("a draft cannot be closed out of hand")
	}
	if !permit.StateInspected.CanMoveTo(permit.StateScheduled) {
		t.Error("a failed inspection goes back to be rescheduled")
	}
	if permit.StateClosed.CanMoveTo(permit.StateDraft) {
		t.Error("nothing follows a closed application")
	}
	if permit.State("pending").CanMoveTo(permit.StateDraft) {
		t.Error("an unknown state goes nowhere")
	}
}

func TestWithdrawalIsAvailableUntilConstruction(t *testing.T) {
	upTo := []permit.State{
		permit.StateDraft, permit.StateSubmitted, permit.StateAccepted,
		permit.StateSurveying, permit.StateEngineered, permit.StateEstimated,
		permit.StateAuthorised, permit.StateScheduled,
	}
	for _, s := range upTo {
		if !s.CanMoveTo(permit.StateWithdrawn) {
			t.Errorf("%s should still allow a withdrawal", s)
		}
	}
	for _, s := range []permit.State{permit.StateConstructed, permit.StateInspected} {
		if s.CanMoveTo(permit.StateWithdrawn) {
			t.Errorf("%s is past the point of withdrawal", s)
		}
	}
}

func TestParseState(t *testing.T) {
	got, err := permit.ParseState(" Estimated ")
	if err != nil || got != permit.StateEstimated {
		t.Errorf("ParseState = %q %v", got, err)
	}
	if _, err := permit.ParseState("pending"); err == nil {
		t.Error("pending should be rejected")
	}
}

func TestHolderOfEachState(t *testing.T) {
	cases := map[permit.State]permit.Holder{
		permit.StateDraft:       permit.HolderAttacher,
		permit.StateSubmitted:   permit.HolderUtility,
		permit.StateEstimated:   permit.HolderAttacher,
		permit.StateScheduled:   permit.HolderUtility,
		permit.StateClosed:      permit.HolderNobody,
		permit.StateRejected:    permit.HolderNobody,
		permit.StateWithdrawn:   permit.HolderNobody,
		permit.State("pending"): permit.HolderNobody,
	}
	for s, want := range cases {
		if got := s.HeldBy(); got != want {
			t.Errorf("%s held by %s, want %s", s, got, want)
		}
	}
}

func TestPathThroughTheMachine(t *testing.T) {
	got, ok := permit.Path(permit.StateDraft, permit.StateClosed)
	if !ok {
		t.Fatal("a draft can reach closed")
	}
	if got[0] != permit.StateDraft || got[len(got)-1] != permit.StateClosed {
		t.Errorf("path = %v", got)
	}
	for i := 1; i < len(got); i++ {
		if !got[i-1].CanMoveTo(got[i]) {
			t.Fatalf("%s does not lead to %s in %v", got[i-1], got[i], got)
		}
	}
	same, ok := permit.Path(permit.StateDraft, permit.StateDraft)
	if !ok || len(same) != 1 {
		t.Errorf("a state reaches itself in one step, got %v", same)
	}
	if _, ok := permit.Path(permit.StateClosed, permit.StateDraft); ok {
		t.Error("nothing leads back out of closed")
	}
	if _, ok := permit.Path(permit.State("pending"), permit.StateDraft); ok {
		t.Error("an unknown state is not on the map")
	}
	if _, ok := permit.Path(permit.StateDraft, permit.State("pending")); ok {
		t.Error("nor is an unknown destination")
	}
}
