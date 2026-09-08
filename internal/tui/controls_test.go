package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestControlFlowStartsAndDrainsImmediately(t *testing.T) {
	api := &controlAPI{
		start: wire.StatusResult{ImplementerCap: 4, Sessions: []wire.SessionResult{{Handle: "ifrit"}, {Handle: "odin"}}},
		stop:  wire.StopResult{Mode: "drain", Sessions: 2, Draining: true},
	}
	flow := NewControlFlow(api)

	start := flow.Update(nil, key("s"))
	if flow.Action.Phase != PhasePending || start == nil || api.startCalls != 0 {
		t.Fatalf("start submit = %#v, calls = %d", flow.Action, api.startCalls)
	}
	startRepair := flow.Update(nil, start())
	if api.startCalls != 1 || flow.Action.Phase != PhaseSuccess || !strings.Contains(flow.View(80), "Implementer cap: 4; sessions: 2") || startRepair == nil || !isRepairSnapshot(startRepair()) {
		t.Fatalf("start result = %#v, calls = %d", flow.Action, api.startCalls)
	}
	flow.Update(nil, key("esc"))

	drain := flow.Update(nil, key("d"))
	if flow.Action.Phase != PhasePending || drain == nil || api.stopCalls != 0 {
		t.Fatalf("drain submit = %#v, calls = %d", flow.Action, api.stopCalls)
	}
	drainRepair := flow.Update(nil, drain())
	if api.stopCalls != 1 || api.stopParams != (wire.StopParams{Hard: false}) || flow.Action.Phase != PhaseSuccess || !strings.Contains(flow.View(80), "Mode: drain; sessions: 2; draining: true") || drainRepair == nil || !isRepairSnapshot(drainRepair()) {
		t.Fatalf("drain result = %#v, calls = %d, params = %#v", flow.Action, api.stopCalls, api.stopParams)
	}
}

func TestControlFlowHardStopRequiresExplicitDestructiveConfirmation(t *testing.T) {
	api := &controlAPI{stop: wire.StopResult{Mode: "hard", Sessions: 0}}
	flow := NewControlFlow(api)
	flow.SetStatus(wire.StatusResult{Sessions: []wire.SessionResult{{Handle: "ifrit"}, {Handle: "odin"}}})

	flow.Update(nil, key("h"))
	view := flow.View(100)
	if flow.Action.Phase != PhaseConfirm || !flow.Action.Destructive || !strings.Contains(view, "terminates running workers") || !strings.Contains(view, "releases their claims") || !strings.Contains(view, "Current sessions: 2") || !strings.Contains(view, "Press c to confirm") {
		t.Fatalf("hard prompt = %#v, %q", flow.Action, view)
	}
	for _, value := range []string{"enter", "x"} {
		if command := flow.Update(nil, key(value)); command != nil || api.stopCalls != 0 || flow.Action.Phase != PhaseConfirm {
			t.Fatalf("key %q called hard stop: command = %v, calls = %d, action = %#v", value, command, api.stopCalls, flow.Action)
		}
	}
	flow.Update(nil, key("esc"))
	if api.stopCalls != 0 || flow.Action.Phase != PhaseIdle {
		t.Fatalf("hard cancel = %#v, calls = %d", flow.Action, api.stopCalls)
	}
	flow.Update(nil, key("h"))
	command := flow.Update(nil, key("c"))
	if command == nil || flow.Action.Phase != PhasePending || api.stopCalls != 0 {
		t.Fatalf("hard submit = %#v, calls = %d", flow.Action, api.stopCalls)
	}
	flow.Update(nil, command())
	if api.stopCalls != 1 || api.stopParams != (wire.StopParams{Hard: true}) || flow.Action.Phase != PhaseSuccess {
		t.Fatalf("hard result = %#v, calls = %d, params = %#v", flow.Action, api.stopCalls, api.stopParams)
	}
}

func TestControlFlowGatesAndConfirmsReview(t *testing.T) {
	reason := "selected bead is not an eligible epic"
	api := &controlAPI{review: wire.ReviewResult{Epic: "magicite-400", Repo: "magicite", Held: true}}
	flow := NewControlFlow(api)
	blocked := dispatchBeads(t, wire.BeadResult{ID: "magicite-1", Repo: "magicite", Review: wire.Eligibility{Reason: &reason}})

	flow.Update(blocked, key("r"))
	hints := flow.Hints(blocked)
	if flow.Action.Phase != PhaseIdle || api.reviewCalls != 0 || len(hints) != 4 || hints[3].Enabled || !strings.Contains(hints[3].Label, reason) {
		t.Fatalf("blocked review = %#v, hints = %#v, calls = %d", flow.Action, hints, api.reviewCalls)
	}

	eligible := dispatchBeads(t, wire.BeadResult{ID: "magicite-400", Repo: "magicite", IssueType: "task", Review: wire.Eligibility{Eligible: true}})
	flow.Update(eligible, key("r"))
	if flow.Action.Phase != PhaseConfirm || !strings.Contains(flow.View(80), "Repository: magicite") || !strings.Contains(flow.View(80), "Epic: magicite-400") || api.reviewCalls != 0 {
		t.Fatalf("review confirmation = %#v, calls = %d", flow.Action, api.reviewCalls)
	}
	command := flow.Update(eligible, key("enter"))
	if command == nil || flow.Action.Phase != PhasePending {
		t.Fatalf("review submit = %#v", flow.Action)
	}
	repair := flow.Update(eligible, command())
	if api.reviewCalls != 1 || api.reviewParams != (wire.ReviewParams{Epic: "magicite-400", Repo: "magicite"}) || flow.Action.Phase != PhaseSuccess || !strings.Contains(flow.View(80), "Epic: magicite-400; held: true") || repair == nil || !isRepairSnapshot(repair()) {
		t.Fatalf("review result = %#v, calls = %d, params = %#v", flow.Action, api.reviewCalls, api.reviewParams)
	}
}

func TestControlFlowSuppressesLateResultsAndRepairsAmbiguity(t *testing.T) {
	api := &controlAPI{err: &transport.Error{Code: transport.ErrorUnavailable}}
	flow := NewControlFlow(api)
	command := flow.Update(nil, key("s"))
	token := flow.Action.Token
	if command == nil {
		t.Fatal("start command = nil")
	}
	if repair := flow.Update(nil, StartResultMsg{Token: token + 1, Result: wire.StatusResult{ImplementerCap: 9}}); repair != nil || flow.Action.Phase != PhasePending {
		t.Fatalf("late result = %#v, action = %#v", repair, flow.Action)
	}
	repair := flow.Update(nil, command())
	if flow.Action.Phase != PhaseFailure || !strings.Contains(flow.View(80), "unavailable") || repair == nil || !isRepairSnapshot(repair()) || api.startCalls != 1 {
		t.Fatalf("ambiguous failure = %#v, calls = %d", flow.Action, api.startCalls)
	}

	api.err = errors.New("daemon replied: exact message")
	flow.Update(nil, key("esc"))
	command = flow.Update(nil, key("s"))
	repair = flow.Update(nil, command())
	if flow.Action.Phase != PhaseFailure || !strings.Contains(flow.View(80), "daemon replied: exact message") || repair == nil || !isRepairSnapshot(repair()) || api.startCalls != 2 {
		t.Fatalf("verbatim failure = %#v, calls = %d", flow.Action, api.startCalls)
	}
}

type controlAPI struct {
	start        wire.StatusResult
	stop         wire.StopResult
	review       wire.ReviewResult
	err          error
	startCalls   int
	stopCalls    int
	reviewCalls  int
	stopParams   wire.StopParams
	reviewParams wire.ReviewParams
}

func (api *controlAPI) Dispatch(context.Context, wire.DispatchParams) (wire.DispatchResult, error) {
	return wire.DispatchResult{}, nil
}

func (api *controlAPI) Start(context.Context) (wire.StatusResult, error) {
	api.startCalls++
	return api.start, api.err
}

func (api *controlAPI) Stop(_ context.Context, params wire.StopParams) (wire.StopResult, error) {
	api.stopCalls++
	api.stopParams = params
	return api.stop, api.err
}

func (api *controlAPI) Review(_ context.Context, params wire.ReviewParams) (wire.ReviewResult, error) {
	api.reviewCalls++
	api.reviewParams = params
	return api.review, api.err
}

var _ ControlAPI = (*controlAPI)(nil)
