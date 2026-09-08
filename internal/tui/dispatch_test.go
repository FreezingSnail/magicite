package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestDispatchFlowPrefillsPickerAndConfirmsBeforeCall(t *testing.T) {
	api := &dispatchAPI{result: wire.DispatchResult{Seat: "ifrit", Role: "repair", Handle: "run-1"}}
	beads := dispatchBeads(t, wire.BeadResult{ID: "magicite-1", Repo: "magicite", Dispatch: wire.Eligibility{Eligible: true}})
	flow := NewDispatchFlow(api)

	flow.Update(beads, key("d"))
	if flow.Action.Phase != PhasePicker || flow.bead.ID != "magicite-1" || flow.bead.Repo != "magicite" {
		t.Fatalf("picker = %#v, bead = %#v", flow.Action, flow.bead)
	}
	if got, want := flow.Roles(), []string{"implement", "design", "repair", "review"}; !equalStrings(got, want) {
		t.Fatalf("roles = %#v, want %#v", got, want)
	}
	flow.Update(beads, key("j"))
	flow.Update(beads, key("j"))
	if flow.Role() != "repair" || !strings.Contains(flow.View(80), "> repair") {
		t.Fatalf("role/view = %q/%q", flow.Role(), flow.View(80))
	}
	flow.Update(beads, key("enter"))
	if flow.Action.Phase != PhaseConfirm || !strings.Contains(flow.Action.View(80), "Repository: magicite") || !strings.Contains(flow.Action.View(80), "Task: magicite-1") || !strings.Contains(flow.Action.View(80), "Role: repair") || api.calls != 0 {
		t.Fatalf("confirmation = %#v, calls = %d", flow.Action, api.calls)
	}
	command := flow.Update(beads, key("enter"))
	if flow.Action.Phase != PhasePending || api.calls != 0 || command == nil {
		t.Fatalf("submission = %#v, calls = %d", flow.Action, api.calls)
	}
	message := command()
	if api.calls != 1 || api.params != (wire.DispatchParams{Task: "magicite-1", Repo: "magicite", Role: "repair"}) {
		t.Fatalf("call = %d, params = %#v", api.calls, api.params)
	}
	repair := flow.Update(beads, message)
	if flow.Action.Phase != PhaseSuccess || !strings.Contains(flow.Action.View(80), "ifrit") || !strings.Contains(flow.Action.View(80), "repair") || !strings.Contains(flow.Action.View(80), "run-1") || repair == nil || !isRepairSnapshot(repair()) {
		t.Fatalf("success = %#v", flow.Action)
	}
}

func TestDispatchFlowGatesIneligibleAndCancelsWithoutCall(t *testing.T) {
	reason := "dependencies incomplete"
	api := &dispatchAPI{}
	beads := dispatchBeads(t, wire.BeadResult{ID: "blocked", Repo: "magicite", Dispatch: wire.Eligibility{Reason: &reason}})
	flow := NewDispatchFlow(api)
	flow.Update(beads, key("d"))
	if flow.Action.Phase != PhaseIdle || api.calls != 0 {
		t.Fatalf("ineligible dispatch = %#v, calls = %d", flow.Action, api.calls)
	}
	hints := flow.Hints(beads)
	if len(hints) != 1 || hints[0].Enabled || !strings.Contains(hints[0].Label, reason) {
		t.Fatalf("ineligible hints = %#v", hints)
	}

	beads = dispatchBeads(t, wire.BeadResult{ID: "ready", Repo: "magicite", Dispatch: wire.Eligibility{Eligible: true}})
	flow.Update(beads, key("d"))
	flow.Update(beads, key("esc"))
	if flow.Action.Phase != PhaseIdle || api.calls != 0 {
		t.Fatalf("picker cancel = %#v, calls = %d", flow.Action, api.calls)
	}
	flow.Update(beads, key("d"))
	flow.Update(beads, key("enter"))
	flow.Update(beads, key("esc"))
	if flow.Action.Phase != PhaseIdle || api.calls != 0 {
		t.Fatalf("confirm cancel = %#v, calls = %d", flow.Action, api.calls)
	}
}

func TestDispatchFlowClassifiesFailuresWithoutRetry(t *testing.T) {
	for _, code := range []transport.ErrorCode{transport.ErrorBadRequest, transport.ErrorConflict, transport.ErrorNotFound, transport.ErrorUnavailable} {
		t.Run(string(code), func(t *testing.T) {
			api := &dispatchAPI{err: &transport.Error{Code: code}}
			beads := dispatchBeads(t, wire.BeadResult{ID: "ready", Repo: "magicite", Dispatch: wire.Eligibility{Eligible: true}})
			flow := NewDispatchFlow(api)
			flow.Update(beads, key("d"))
			flow.Update(beads, key("enter"))
			message := flow.Update(beads, key("enter"))()
			repair := flow.Update(beads, message)
			if flow.Action.Phase != PhaseFailure || !strings.Contains(flow.Action.Result, string(code)) || api.calls != 1 {
				t.Fatalf("failure = %#v, calls = %d", flow.Action, api.calls)
			}
			if code == transport.ErrorUnavailable {
				if repair == nil || !isRepairSnapshot(repair()) {
					t.Fatalf("unavailable did not repair: %#v", repair)
				}
			} else if repair != nil {
				t.Fatalf("known failure repaired: %#v", repair)
			}
		})
	}
}

func TestDispatchFailureTextPreservesUnclassifiedMessage(t *testing.T) {
	if got := dispatchFailureText(errors.New("socket closed")); !strings.Contains(got, "socket closed") {
		t.Fatalf("failure = %q", got)
	}
}

func dispatchBeads(t *testing.T, bead wire.BeadResult) *BeadsView {
	t.Helper()
	beads := NewBeadsView()
	beads.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{bead}})
	return beads
}

func key(value string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)} }

type dispatchAPI struct {
	calls  int
	params wire.DispatchParams
	result wire.DispatchResult
	err    error
}

func (api *dispatchAPI) Dispatch(_ context.Context, params wire.DispatchParams) (wire.DispatchResult, error) {
	api.calls++
	api.params = params
	return api.result, api.err
}

var _ DaemonAPI = (*dispatchAPI)(nil)
