package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

func TestActionStateTransitionsRejectsDuplicateAndStaleResults(t *testing.T) {
	var state ActionState
	state.Begin("Dispatch", []string{"Task: bead-1"})
	if state.Phase != PhaseConfirm || state.Label != "Dispatch" || state.Result != "" || state.Err != nil {
		t.Fatalf("Begin state = %#v", state)
	}
	token := state.Submit()
	if token == 0 || state.Phase != PhasePending {
		t.Fatalf("Submit state = %#v", state)
	}
	if duplicate := state.Submit(); duplicate != 0 {
		t.Fatalf("duplicate token = %d, want 0", duplicate)
	}
	if message := state.Resolve(token+1, "late", nil); message != nil || state.Phase != PhasePending {
		t.Fatalf("stale resolution = %#v, state %#v", message, state)
	}
	if message := state.Resolve(token, "done", nil); !isRepairSnapshot(message) || state.Phase != PhaseSuccess || state.Result != "done" {
		t.Fatalf("success resolution = %#v, state %#v", message, state)
	}
	state.Dismiss()
	if state.Phase != PhaseIdle || state.Result != "" || state.Err != nil {
		t.Fatalf("Dismiss state = %#v", state)
	}
	state.Begin("Next", nil)
	next := state.Submit()
	if message := state.Resolve(token, "old", nil); message != nil || state.Phase != PhasePending || state.Token != next {
		t.Fatalf("dismissed request resolved: %#v, state %#v", message, state)
	}
}

func TestActionStateHintsDestructiveAndRepairPolicy(t *testing.T) {
	var state ActionState
	state.Begin("Hard stop", []string{"Terminates workers"})
	state.Destructive = true
	if got := state.Hints(); len(got) != 2 || got[0].Key != "c" || strings.Contains(state.View(80), "Enter confirm") || !strings.Contains(state.View(80), "Press c to confirm") {
		t.Fatalf("destructive confirmation = %#v, %q", got, state.View(80))
	}
	token := state.Submit()
	if message := state.Resolve(token, "", &transport.Error{Code: transport.ErrorConflict}); message != nil || state.Phase != PhaseFailure {
		t.Fatalf("conflict resolution = %#v, state %#v", message, state)
	}
	state.Begin("Dispatch", nil)
	token = state.Submit()
	if message := state.Resolve(token, "", &transport.Error{Code: transport.ErrorUnavailable}); !isRepairSnapshot(message) {
		t.Fatalf("unavailable repair = %#v", message)
	}
	state.Begin("Dispatch", nil)
	token = state.Submit()
	if message := state.Resolve(token, "", errors.New("connection reset")); !isRepairSnapshot(message) {
		t.Fatalf("unknown transport repair = %#v", message)
	}
}

func isRepairSnapshot(message any) bool {
	_, ok := message.(RepairSnapshotMsg)
	return ok
}
