package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/FreezingSnail/magicite/internal/repo"
)

// ErrUnknownRole reports a plan request for an unsupported dispatch role.
var ErrUnknownRole = errors.New("dispatch: unknown role")

// PlanError identifies a role plan construction failure.
type PlanError struct {
	Role Role
	Task string
	Err  error
}

func (e *PlanError) Error() string {
	return fmt.Sprintf("dispatch: plan for %s task %q: %v", e.Role, e.Task, e.Err)
}

func (e *PlanError) Unwrap() error { return e.Err }

// BDInputRules is the safe protocol for filing prose with bd.
const BDInputRules = `SHELL-SAFE BD INPUT (MANDATORY)

Never inline prose in a bd flag. Your shell may be fish, which has no heredocs (<<'EOF') and does not read POSIX quoting the same way bash does, so quoted multi-line text, apostrophes, backslashes and $ get mangled or the command hangs. Write the text to a file with your file-write tool, then point bd at the file:

  bd create "<short title>" --type task --parent <epic-id> \
    --body-file /tmp/<name>-desc.md \
    --design-file /tmp/<name>-design.md \
    --labels staged,difficulty:low --silent

  bd update <id> --body-file /tmp/<name>-desc.md \
                 --design-file /tmp/<name>-design.md

Rules:
- Description and design ALWAYS travel via --body-file / --design-file (- reads stdin). Never -d/--description or --design with inline prose.
- Titles and --acceptance have no file flag: keep them to one line of plain ASCII with no apostrophes, no double quotes, no backticks, no backslashes, and no $. Put anything richer in the design or description file.
- Never use a heredoc. Never build a bd command with nested quotes.
- Write files with your file-write tool, not with echo/printf redirection.
- Verify with bd show <id> after writing: if a field is truncated or contains escape characters, the quoting broke — rewrite it from a file.`

// SpecSections renders non-empty task fields in their contract order.
func SpecSections(spec Spec) string {
	var sections []string
	for _, field := range []struct {
		label string
		value string
	}{
		{"Title", spec.Title},
		{"Description", spec.Description},
		{"Design", spec.Design},
		{"Acceptance", spec.Acceptance},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" {
			continue
		}
		sections = append(sections, field.label+":\n"+value)
	}
	return strings.Join(sections, "\n\n")
}

// PlanFor builds the agent-facing plan for one role.
func (d *Dispatcher) PlanFor(ctx context.Context, repository repo.Repo, role Role, task, seat string) (string, error) {
	if role == Reviewer {
		run, err := d.gate.ReviewPlan(ctx, repository, task)
		if err != nil {
			return "", &PlanError{Role: role, Task: task, Err: err}
		}
		return run.Plan, nil
	}

	if role != Implementer && role != Designer && role != Repairer {
		return "", &PlanError{Role: role, Task: task, Err: ErrUnknownRole}
	}

	// An unavailable specification must not prevent a useful task dispatch.
	spec, _ := d.beads.Show(ctx, repository, task)
	var plan string
	switch role {
	case Implementer:
		plan = implementerPlan(repository, task, seat, spec)
	case Designer:
		plan = designerPlan(repository, task, seat, spec)
	case Repairer:
		plan = repairerPlan(repository, task, seat)
	}
	return plan, nil
}

func implementerPlan(repository repo.Repo, task, seat string, spec Spec) string {
	return joinPlan(
		"You are the implementer for task "+task+" in repository "+repository.Name+".",
		"Implement the task from the specification below. Commit the change to the "+seat+" seat branch. Describe what changed in the commit message body; that message is the task record.",
		"Write no summary or report files. If blocked, report the blocker instead of inventing work.",
		specBlock(spec),
		BDInputRules,
	)
}

func designerPlan(repository repo.Repo, task, seat string, spec Spec) string {
	return joinPlan(
		"You are the designer for task "+task+" in repository "+repository.Name+".",
		"Produce the design for the task. Record the design and acceptance criteria on the bead. Work from the specification below and report blockers instead of inventing work.",
		specBlock(spec),
		BDInputRules,
	)
}

func repairerPlan(repository repo.Repo, task, seat string) string {
	return joinPlan(
		"You are the repairer for task "+task+" in repository "+repository.Name+" on seat "+seat+".",
		"The integration branch is "+repository.Branch+". Rebase the seat branch onto the integration branch. Resolve every conflict, run git add -A, then run git rebase --continue. Repeat that sequence until the rebase finishes. Never merge the integration branch into the seat branch.",
	)
}

func specBlock(spec Spec) string {
	sections := SpecSections(spec)
	if sections == "" {
		return "Task specification: no readable specification was provided; use task context and report blockers rather than inventing requirements."
	}
	return "Task specification:\n\n" + sections
}

func joinPlan(parts ...string) string {
	var nonEmpty []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}
