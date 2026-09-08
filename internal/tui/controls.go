package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const controlTimeout = 10 * time.Second

// ControlAPI is the daemon control capability used by lifecycle and review
// flows. It embeds the existing action API without adding any TUI-only RPCs.
type ControlAPI interface {
	DaemonAPI
	Start(context.Context) (wire.StatusResult, error)
	Stop(context.Context, wire.StopParams) (wire.StopResult, error)
	Review(context.Context, wire.ReviewParams) (wire.ReviewResult, error)
}

// StartResultMsg delivers one start RPC outcome to Update.
type StartResultMsg struct {
	Token  uint64
	Result wire.StatusResult
	Err    error
}

// StopResultMsg delivers one stop RPC outcome to Update.
type StopResultMsg struct {
	Token  uint64
	Result wire.StopResult
	Err    error
}

// ReviewResultMsg delivers one review RPC outcome to Update.
type ReviewResultMsg struct {
	Token  uint64
	Result wire.ReviewResult
	Err    error
}

// ControlFlow owns lifecycle and review actions for the current snapshot.
type ControlFlow struct {
	API    ControlAPI
	Action ActionState

	status wire.StatusResult
	bead   wire.BeadResult
	kind   controlKind
}

type controlKind string

const (
	controlStart  controlKind = "start"
	controlDrain  controlKind = "drain"
	controlHard   controlKind = "hard-stop"
	controlReview controlKind = "review"
)

// NewControlFlow constructs an idle lifecycle and review flow.
func NewControlFlow(api ControlAPI) *ControlFlow {
	return &ControlFlow{API: api, Action: ActionState{Phase: PhaseIdle}}
}

// SetStatus retains the daemon status used by destructive confirmation text.
func (flow *ControlFlow) SetStatus(status wire.StatusResult) { flow.status = status }

// Snapshot retains current daemon status. Snapshot sessions take precedence
// when supplied because they are the coherent action-time session inventory.
func (flow *ControlFlow) Snapshot(snapshot wire.SnapshotResult) {
	flow.status = snapshot.Runtime
	if snapshot.Sessions != nil {
		flow.status.Sessions = append([]wire.SessionResult(nil), snapshot.Sessions...)
	}
}

// OpenReview opens a confirmation for the daemon-declared eligible selected bead.
func (flow *ControlFlow) OpenReview(beads *BeadsView) bool {
	if beads == nil {
		return false
	}
	bead, ok := beads.SelectedBead()
	if !ok || !bead.Review.Eligible {
		return false
	}
	flow.bead = bead
	flow.kind = controlReview
	flow.Action.Begin("Review", flow.identityPrompt())
	return true
}

// Hints lists effective lifecycle keys or the selected bead's review state.
func (flow *ControlFlow) Hints(beads *BeadsView) []Hint {
	if flow.Action.Phase != PhaseIdle {
		return flow.Action.Hints()
	}
	hints := []Hint{
		{Key: "s", Label: "start", Enabled: true},
		{Key: "d", Label: "drain", Enabled: true},
		{Key: "h", Label: "hard stop", Enabled: true},
	}
	if beads == nil {
		return append(hints, Hint{Key: "r", Label: "review", Enabled: false})
	}
	bead, ok := beads.SelectedBead()
	if !ok || !bead.Review.Eligible {
		label := "review unavailable"
		if ok && bead.Review.Reason != nil && *bead.Review.Reason != "" {
			label += ": " + SanitizeText(*bead.Review.Reason)
		}
		return append(hints, Hint{Key: "r", Label: label, Enabled: false})
	}
	return append(hints, Hint{Key: "r", Label: "review", Enabled: true})
}

// View renders the shared action modal from cached state.
func (flow *ControlFlow) View(width int) string { return flow.Action.View(width) }

// Update handles lifecycle/review keys and their asynchronous outcomes.
func (flow *ControlFlow) Update(beads *BeadsView, message tea.Msg) tea.Cmd {
	switch message := message.(type) {
	case StartResultMsg:
		return flow.resolve(message.Token, startResultText(message.Result), message.Err)
	case StopResultMsg:
		return flow.resolve(message.Token, stopResultText(message.Result), message.Err)
	case ReviewResultMsg:
		return flow.resolve(message.Token, reviewResultText(message.Result), message.Err)
	case tea.KeyMsg:
		return flow.updateKey(beads, message)
	default:
		return nil
	}
}

func (flow *ControlFlow) updateKey(beads *BeadsView, key tea.KeyMsg) tea.Cmd {
	switch flow.Action.Phase {
	case PhaseIdle:
		switch key.String() {
		case "s":
			flow.kind = controlStart
			return flow.submitImmediate("Start")
		case "d":
			flow.kind = controlDrain
			return flow.submitImmediate("Drain")
		case "h":
			flow.kind = controlHard
			flow.Action.Begin("Hard stop", []string{
				"Immediately terminates running workers and releases their claims.",
				fmt.Sprintf("Current sessions: %d", len(flow.status.Sessions)),
			})
			flow.Action.Destructive = true
		case "r":
			flow.OpenReview(beads)
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

func (flow *ControlFlow) submitImmediate(label string) tea.Cmd {
	flow.Action.Begin(label, nil)
	return flow.submit()
}

func (flow *ControlFlow) submit() tea.Cmd {
	token := flow.Action.Submit()
	if token == 0 {
		return nil
	}
	api, kind, bead := flow.API, flow.kind, flow.bead
	return func() tea.Msg {
		if api == nil {
			return controlUnavailable(kind, token)
		}
		ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
		defer cancel()
		switch kind {
		case controlStart:
			result, err := api.Start(ctx)
			return StartResultMsg{Token: token, Result: result, Err: err}
		case controlDrain:
			result, err := api.Stop(ctx, wire.StopParams{Hard: false})
			return StopResultMsg{Token: token, Result: result, Err: err}
		case controlHard:
			result, err := api.Stop(ctx, wire.StopParams{Hard: true})
			return StopResultMsg{Token: token, Result: result, Err: err}
		case controlReview:
			result, err := api.Review(ctx, wire.ReviewParams{Epic: bead.ID, Repo: bead.Repo})
			return ReviewResultMsg{Token: token, Result: result, Err: err}
		default:
			return controlUnavailable(kind, token)
		}
	}
}

func controlUnavailable(kind controlKind, token uint64) tea.Msg {
	err := &transport.Error{Code: transport.ErrorUnavailable}
	switch kind {
	case controlStart:
		return StartResultMsg{Token: token, Err: err}
	case controlDrain, controlHard:
		return StopResultMsg{Token: token, Err: err}
	default:
		return ReviewResultMsg{Token: token, Err: err}
	}
}

func (flow *ControlFlow) resolve(token uint64, result string, err error) tea.Cmd {
	if err != nil {
		result = controlFailureText(err)
	}
	resolved := flow.Action.Resolve(token, result, err)
	if resolved == nil {
		return nil
	}
	return func() tea.Msg { return resolved }
}

func (flow *ControlFlow) identityPrompt() []string {
	return []string{"Repository: " + flow.bead.Repo, "Epic: " + flow.bead.ID}
}

func startResultText(result wire.StatusResult) string {
	return fmt.Sprintf("Implementer cap: %d; sessions: %d", result.ImplementerCap, len(result.Sessions))
}

func stopResultText(result wire.StopResult) string {
	return fmt.Sprintf("Mode: %s; sessions: %d; draining: %t", result.Mode, result.Sessions, result.Draining)
}

func reviewResultText(result wire.ReviewResult) string {
	return fmt.Sprintf("Epic: %s; held: %t", result.Epic, result.Held)
}

func controlFailureText(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
