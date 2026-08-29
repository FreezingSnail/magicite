package dispatch

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/FreezingSnail/magicite/internal/logging"
	stampdata "github.com/FreezingSnail/magicite/internal/stamp"
)

// OnComplete handles one spawned session's terminal runtime outcome.
func (d *Dispatcher) OnComplete(ctx context.Context, handle string, outcome Outcome) {
	session, ok := d.Remove(handle)
	if !ok {
		return
	}

	fields := dispatchFields(session.Repo, session.Task, session.Role, session.Seat)
	fields["handle"] = handle
	fields["outcome"] = outcome
	d.log(logging.Info, logging.KindComplete, fields)

	completed := false
	deferred := func() {
		if recovered := recover(); recovered != nil {
			d.terminalFailure(ctx, session, fmt.Sprintf("session routing panicked: %v", recovered))
		}
		_ = d.runner.Delete(ctx, handle)
		d.completeDrain()
	}
	defer deferred()

	if outcome == Completed {
		completed = d.complete(ctx, session)
	}
	if !completed {
		d.fail(ctx, session, outcome)
	}
}

// Drained reports whether all spawned sessions have finished.
func (d *Dispatcher) Drained() bool { return d.Idle() }

func (d *Dispatcher) complete(ctx context.Context, session Session) bool {
	if session.Role == Reviewer {
		output, err := d.runner.Output(ctx, session.Handle)
		if err != nil {
			d.abortReview(ctx, session, err.Error())
			return true
		}
		if err := d.gate.CompleteReview(ctx, session.Handle, output); err != nil {
			d.abortReview(ctx, session, err.Error())
		}
		return true
	}

	provenance := d.StampFor(session)
	diffs, err := d.runner.Diff(ctx, session.Handle)
	if err != nil {
		d.terminalFailure(ctx, session, "session failed; task is left open.")
		return true
	}
	result, err := d.lander.Land(ctx, session.Repo, session.Seat, provenance)
	if err != nil {
		d.landFailure(ctx, session, "task is left open after landing failed.")
		return true
	}

	switch result {
	case LandOK:
		return d.landed(ctx, session, d.FormatDiffs(diffs), provenance)
	case LandConflict:
		d.landConflict(ctx, session)
		return true
	default:
		d.landFailure(ctx, session, "task is left open after landing failed.")
		return true
	}
}

func (d *Dispatcher) landed(ctx context.Context, session Session, closeOutput string, provenance Stamp) bool {
	if session.Role == Designer {
		if session.Decomposition {
			if _, err := d.gate.DecompositionVerdict(ctx, session.Repo, session.Task); err != nil {
				d.log(logging.Warn, logging.KindVerdict, map[string]any{"repo": session.Repo.LogName(), "epic": session.Task, "error": err.Error()})
			}
		}
		return true
	}

	landed, err := d.lander.Landed(ctx, session.Repo, session.Seat)
	if err != nil || !landed {
		d.landFailure(ctx, session, "task is left open because its branch was not landed.")
		return true
	}
	taskLanded, err := d.lander.TaskLanded(ctx, session.Repo, session.Task)
	if err != nil || !taskLanded {
		d.landFailure(ctx, session, "task is left open because its provenance was not landed.")
		return true
	}

	closeOutput = appendTrailers(closeOutput, provenance)
	if err := d.beads.Close(ctx, session.Repo, session.Task, closeOutput); err != nil {
		d.landFailure(ctx, session, "task is left open because closing it failed.")
		return true
	}
	d.log(logging.Info, logging.KindClose, dispatchFields(session.Repo, session.Task, session.Role, session.Seat))

	epic, err := d.gate.DueEpic(ctx, session.Repo, session.Task)
	if err != nil || epic == "" {
		return true
	}
	d.Review(ctx, session.Repo, epic)
	return true
}

func (d *Dispatcher) fail(ctx context.Context, session Session, outcome Outcome) {
	if d.FallbackRetry(ctx, session, session.Handle, outcome) {
		return
	}
	if session.Role == Reviewer {
		d.abortReview(ctx, session, string(outcome))
		return
	}
	d.terminalFailure(ctx, session, "session failed; task is left open.")
}

func (d *Dispatcher) landConflict(ctx context.Context, session Session) {
	d.log(logging.Warn, logging.KindLand, map[string]any{"repo": session.Repo.LogName(), "task": session.Task, "seat": session.Seat, "result": LandConflict})
	_ = d.beads.Comment(ctx, session.Repo, session.Task, "landing conflicted; a repairer was dispatched.")
	if session.Role != Repairer {
		d.Repair(ctx, session.Repo, session.Seat, session.Task)
	}
}

func (d *Dispatcher) landFailure(ctx context.Context, session Session, comment string) {
	d.log(logging.Warn, logging.KindLand, map[string]any{"repo": session.Repo.LogName(), "task": session.Task, "seat": session.Seat, "result": "left-open"})
	_ = d.beads.Comment(ctx, session.Repo, session.Task, comment)
	_ = d.beads.Release(ctx, session.Repo, session.Task)
}

func (d *Dispatcher) terminalFailure(ctx context.Context, session Session, comment string) {
	d.log(logging.Warn, "session-failed", dispatchFields(session.Repo, session.Task, session.Role, session.Seat))
	_ = d.beads.Comment(ctx, session.Repo, session.Task, comment)
	_ = d.beads.Release(ctx, session.Repo, session.Task)
}

func (d *Dispatcher) abortReview(ctx context.Context, session Session, reason string) {
	_ = d.gate.AbortReview(ctx, session.Handle, reason)
	d.log(logging.Warn, logging.KindReview, map[string]any{"repo": session.Repo.LogName(), "epic": session.Task, "handle": session.Handle, "error": reason})
}

// StampFor builds the provenance attached to a completed session's commit.
func (d *Dispatcher) StampFor(session Session) Stamp {
	return Stamp{
		Model: session.Model, Backend: session.Backend, Difficulty: session.Difficulty,
		Effort: session.Effort, Agent: session.Agent, Repo: session.Repo.Name,
		Seat: session.Seat, Task: session.Task,
		Harness:    d.config.Harness.Name + " " + d.config.Harness.Version,
		HarnessRev: shortHEAD(session.Repo.Root),
	}
}

func shortHEAD(root string) string {
	output, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// FormatDiffs renders runtime changes for a task close record.
func (d *Dispatcher) FormatDiffs(diffs []Diff) string {
	if len(diffs) == 0 {
		return "Landed with no reported file changes."
	}
	var output strings.Builder
	output.WriteString("Landed changes:\n")
	for _, diff := range diffs {
		fmt.Fprintf(&output, "- %s (%s, +%d -%d)", diff.File, diff.Status, diff.Additions, diff.Deletions)
		if patch := strings.TrimSpace(diff.Patch); patch != "" {
			output.WriteString("\n")
			output.WriteString(patch)
		}
		output.WriteString("\n")
	}
	return strings.TrimSpace(output.String())
}

func appendTrailers(output string, provenance Stamp) string {
	trailers := stampdata.Stamp{
		Model: provenance.Model, Backend: provenance.Backend, Difficulty: provenance.Difficulty,
		Effort: provenance.Effort, Agent: provenance.Agent, Repo: provenance.Repo,
		Seat: provenance.Seat, Task: provenance.Task, Harness: provenance.Harness,
		HarnessRev: provenance.HarnessRev,
	}.Trailers()
	return stampdata.Apply(output, trailers)
}
