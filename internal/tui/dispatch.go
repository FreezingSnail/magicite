package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const dispatchTimeout = 10 * time.Second

var dispatchRoles = []string{"implement", "design", "repair", "review"}

// DaemonAPI is the control capability required by TUI action flows.
type DaemonAPI interface {
	Dispatch(context.Context, wire.DispatchParams) (wire.DispatchResult, error)
}

// DispatchResultMsg delivers one confirmed dispatch RPC outcome to Update.
type DispatchResultMsg struct {
	Token  uint64
	Result wire.DispatchResult
	Err    error
}

// DispatchFlow controls dispatch for the currently selected bead. It keeps
// daemon eligibility authoritative and never mutates the selected bead.
type DispatchFlow struct {
	API    DaemonAPI
	Action ActionState

	bead      wire.BeadResult
	roleIndex int
}

// NewDispatchFlow constructs an idle dispatch flow.
func NewDispatchFlow(api DaemonAPI) *DispatchFlow {
	return &DispatchFlow{API: api, Action: ActionState{Phase: PhaseIdle}}
}

// Roles returns the exact dispatch role vocabulary accepted by the daemon.
func (*DispatchFlow) Roles() []string { return append([]string(nil), dispatchRoles...) }

// OpenSelected opens the role picker from the Beads tab's stable selection.
// It returns false when no bead is selected or the daemon marks it ineligible.
func (flow *DispatchFlow) OpenSelected(beads *BeadsView) bool {
	if beads == nil {
		return false
	}
	bead, ok := beads.SelectedBead()
	if !ok || !bead.Dispatch.Eligible {
		return false
	}
	flow.bead = bead
	flow.roleIndex = 0
	flow.Action.OpenPicker("Dispatch", flow.identityPrompt())
	return true
}

// Hints returns the dispatch key when idle, including the daemon's exact
// ineligibility reason, or the currently effective modal controls.
func (flow *DispatchFlow) Hints(beads *BeadsView) []Hint {
	if flow.Action.Phase != PhaseIdle {
		return flow.Action.Hints()
	}
	if beads == nil {
		return []Hint{{Key: "d", Label: "dispatch", Enabled: false}}
	}
	bead, ok := beads.SelectedBead()
	if !ok {
		return []Hint{{Key: "d", Label: "dispatch", Enabled: false}}
	}
	if !bead.Dispatch.Eligible {
		label := "dispatch unavailable"
		if bead.Dispatch.Reason != nil && *bead.Dispatch.Reason != "" {
			label += ": " + SanitizeText(*bead.Dispatch.Reason)
		}
		return []Hint{{Key: "d", Label: label, Enabled: false}}
	}
	return []Hint{{Key: "d", Label: "dispatch", Enabled: true}}
}

// Update handles a key or dispatch result. Returned commands only begin after
// a confirmation and are never retried by this flow.
func (flow *DispatchFlow) Update(beads *BeadsView, message tea.Msg) tea.Cmd {
	switch message := message.(type) {
	case DispatchResultMsg:
		result := dispatchResultText(message.Result)
		if message.Err != nil {
			result = dispatchFailureText(message.Err)
		}
		resolved := flow.Action.Resolve(message.Token, result, message.Err)
		if resolved == nil {
			return nil
		}
		return func() tea.Msg { return resolved }
	case tea.KeyMsg:
		return flow.updateKey(beads, message)
	default:
		return nil
	}
}

func (flow *DispatchFlow) updateKey(beads *BeadsView, key tea.KeyMsg) tea.Cmd {
	switch flow.Action.Phase {
	case PhaseIdle:
		if key.String() == "d" {
			flow.OpenSelected(beads)
		}
	case PhasePicker:
		switch key.String() {
		case "j", "down":
			flow.roleIndex = (flow.roleIndex + 1) % len(dispatchRoles)
		case "k", "up":
			flow.roleIndex = (flow.roleIndex + len(dispatchRoles) - 1) % len(dispatchRoles)
		case "enter":
			flow.Action.Begin("Dispatch", append(flow.identityPrompt(), "Role: "+flow.Role()))
		case "esc":
			flow.Action.Dismiss()
		}
	case PhaseConfirm:
		switch key.String() {
		case "esc":
			flow.Action.Dismiss()
		case "enter":
			if !flow.Action.Destructive {
				return flow.submit()
			}
		case "c":
			if flow.Action.Destructive {
				return flow.submit()
			}
		}
	case PhaseSuccess, PhaseFailure:
		if key.String() == "esc" {
			flow.Action.Dismiss()
		}
	}
	return nil
}

func (flow *DispatchFlow) submit() tea.Cmd {
	token := flow.Action.Submit()
	if token == 0 {
		return nil
	}
	params := wire.DispatchParams{Task: flow.bead.ID, Repo: flow.bead.Repo, Role: flow.Role()}
	api := flow.API
	return func() tea.Msg {
		if api == nil {
			return DispatchResultMsg{Token: token, Err: &transport.Error{Code: transport.ErrorUnavailable}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
		defer cancel()
		result, err := api.Dispatch(ctx, params)
		return DispatchResultMsg{Token: token, Result: result, Err: err}
	}
}

// Role returns the selected daemon wire role.
func (flow *DispatchFlow) Role() string { return dispatchRoles[flow.roleIndex] }

// View renders the role picker or the shared action modal from cached state.
func (flow *DispatchFlow) View(width int) string {
	if flow.Action.Phase != PhasePicker {
		return flow.Action.View(width)
	}
	if width <= 0 {
		return ""
	}
	lines := append([]string{"Dispatch"}, flow.identityPrompt()...)
	for index, role := range dispatchRoles {
		prefix := "  "
		if index == flow.roleIndex {
			prefix = "> "
		}
		lines = append(lines, prefix+role)
	}
	lines = append(lines, "Enter continue", "Esc cancel")
	for index := range lines {
		lines[index] = Truncate(SanitizeText(lines[index]), width)
	}
	return strings.Join(lines, "\n")
}

func (flow *DispatchFlow) identityPrompt() []string {
	return []string{"Repository: " + flow.bead.Repo, "Task: " + flow.bead.ID}
}

func dispatchResultText(result wire.DispatchResult) string {
	return fmt.Sprintf("Dispatched seat %s as %s (handle %s)", result.Seat, result.Role, result.Handle)
}

func dispatchFailureText(err error) string {
	if err == nil {
		return ""
	}
	var transportError *transport.Error
	if !errorAs(err, &transportError) {
		return "Dispatch failed: " + err.Error()
	}
	label := "dispatch failed"
	switch transportError.Code {
	case transport.ErrorBadRequest:
		label = "bad request"
	case transport.ErrorConflict:
		label = "conflict"
	case transport.ErrorNotFound:
		label = "not found"
	case transport.ErrorUnavailable:
		label = "unavailable"
	}
	return fmt.Sprintf("Dispatch %s: %s [%s]", label, err.Error(), transportError.Code)
}

func errorAs(err error, target any) bool {
	return errors.As(err, target)
}
