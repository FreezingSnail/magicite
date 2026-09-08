package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

// ActionPhase identifies the visible state of one asynchronous control action.
type ActionPhase string

const (
	PhaseIdle    ActionPhase = ""
	PhasePicker  ActionPhase = "picker"
	PhaseConfirm ActionPhase = "confirm"
	PhasePending ActionPhase = "pending"
	PhaseSuccess ActionPhase = "success"
	PhaseFailure ActionPhase = "failure"
)

// RepairSnapshotMsg asks the runtime to repair daemon-owned state after an
// action may have changed it or a transport failure made its outcome unknown.
type RepairSnapshotMsg struct{}

// ActionState retains a single modal action lifecycle. Update owners call its
// methods; View only reads the state and never starts I/O.
type ActionState struct {
	Phase       ActionPhase
	Label       string
	Prompt      []string
	Token       uint64
	Result      string
	Err         error
	Destructive bool
}

// Begin opens a non-destructive confirmation, replacing any prior outcome.
func (state *ActionState) Begin(label string, prompt []string) {
	state.Phase = PhaseConfirm
	state.Label = label
	state.Prompt = append(state.Prompt[:0], prompt...)
	state.Result = ""
	state.Err = nil
	state.Destructive = false
}

// OpenPicker opens a picker before its eventual confirmation.
func (state *ActionState) OpenPicker(label string, prompt []string) {
	state.Phase = PhasePicker
	state.Label = label
	state.Prompt = append(state.Prompt[:0], prompt...)
	state.Result = ""
	state.Err = nil
	state.Destructive = false
}

// Submit advances a confirmation to pending and returns its unique request
// token. Zero means no submission was accepted.
func (state *ActionState) Submit() uint64 {
	if state.Phase != PhaseConfirm {
		return 0
	}
	state.Token++
	state.Phase = PhasePending
	return state.Token
}

// Resolve accepts only the active pending request. It requests a snapshot
// repair after success and after errors whose outcome may be unknown.
func (state *ActionState) Resolve(token uint64, result string, err error) tea.Msg {
	if state.Phase != PhasePending || token != state.Token {
		return nil
	}
	state.Result = result
	state.Err = err
	if err == nil {
		state.Phase = PhaseSuccess
	} else {
		state.Phase = PhaseFailure
	}
	if err == nil || ambiguousTransportFailure(err) {
		return RepairSnapshotMsg{}
	}
	return nil
}

// Dismiss closes a picker, confirmation, or result and invalidates any prior
// request token so a late callback cannot repaint a later action.
func (state *ActionState) Dismiss() {
	state.Token++
	state.Phase = PhaseIdle
	state.Label = ""
	state.Prompt = nil
	state.Result = ""
	state.Err = nil
	state.Destructive = false
}

// Hints lists only keys which can affect the current action state.
func (state ActionState) Hints() []Hint {
	switch state.Phase {
	case PhasePicker:
		return []Hint{{Key: "j/k", Label: "select", Enabled: true}, {Key: "enter", Label: "continue", Enabled: true}, {Key: "esc", Label: "cancel", Enabled: true}}
	case PhaseConfirm:
		confirm := "enter"
		if state.Destructive {
			confirm = "c"
		}
		return []Hint{{Key: confirm, Label: "confirm", Enabled: true}, {Key: "esc", Label: "cancel", Enabled: true}}
	case PhaseSuccess, PhaseFailure:
		return []Hint{{Key: "esc", Label: "dismiss", Enabled: true}}
	default:
		return nil
	}
}

// View renders a bounded confirmation or result modal from already-held data.
func (state ActionState) View(width int) string {
	if width <= 0 || state.Phase == PhaseIdle || state.Phase == PhasePicker {
		return ""
	}
	lines := []string{SanitizeText(state.Label)}
	for _, prompt := range state.Prompt {
		lines = append(lines, SanitizeText(prompt))
	}
	switch state.Phase {
	case PhaseConfirm:
		if state.Destructive {
			lines = append(lines, "Press c to confirm", "Esc cancel")
		} else {
			lines = append(lines, "Enter confirm", "Esc cancel")
		}
	case PhasePending:
		lines = append(lines, "Pending…")
	case PhaseSuccess:
		lines = append(lines, SanitizeText(state.Result), "Esc dismiss")
	case PhaseFailure:
		if state.Err != nil {
			lines = append(lines, SanitizeText(state.Err.Error()))
		}
		lines = append(lines, "Esc dismiss")
	}
	for index := range lines {
		lines[index] = Truncate(lines[index], width)
	}
	return strings.Join(lines, "\n")
}

func ambiguousTransportFailure(err error) bool {
	var transportError *transport.Error
	if !errors.As(err, &transportError) {
		return true
	}
	switch transportError.Code {
	case transport.ErrorUnavailable, transport.ErrorInternal:
		return true
	default:
		return false
	}
}
