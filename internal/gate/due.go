package gate

import (
	"context"
	"fmt"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/repo"
)

const (
	retryExhaustedComment = "review retry budget exhausted after %d attempts; human attention is needed."
	disabledCloseReason   = "all child tasks closed; review gate disabled."
)

// DriftFixTasks returns open non-human drift-fix task IDs for r.
func (g *Gate) DriftFixTasks(ctx context.Context, r repo.Repo) ([]string, error) {
	if !g.Enabled() {
		return []string{}, nil
	}
	beads, err := g.beads.Query(ctx, r, bd.DriftFixQuery())
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(beads))
	for _, bead := range beads {
		ids = append(ids, bead.ID)
	}
	return ids, nil
}

// HoldWith reports whether r must wait for review or drift remediation.
func (g *Gate) HoldWith(r repo.Repo, driftFixes []string) bool {
	if !g.Enabled() {
		return false
	}
	if _, ok := g.key(r, "hold"); !ok {
		return true
	}
	return g.inFlight(r) || len(driftFixes) != 0
}

// Hold queries drift fixes and reports whether r must wait for the gate.
func (g *Gate) Hold(ctx context.Context, r repo.Repo) (bool, error) {
	driftFixes, err := g.DriftFixTasks(ctx, r)
	if err != nil {
		return true, err
	}
	return g.HoldWith(r, driftFixes), nil
}

// ChildrenClosed reports whether epic has at least one child and all are closed.
func (g *Gate) ChildrenClosed(ctx context.Context, r repo.Repo, epic string) (bool, error) {
	children, err := g.beads.EpicChildren(ctx, r, epic)
	if err != nil {
		return false, err
	}
	if len(children) == 0 {
		return false, nil
	}
	for _, child := range children {
		if child.Status != "closed" {
			return false, nil
		}
	}
	return true, nil
}

// DueEpic returns task's completed parent epic when it needs a review.
func (g *Gate) DueEpic(ctx context.Context, r repo.Repo, task string) (string, error) {
	if !g.Enabled() {
		return "", nil
	}
	bead, err := g.beads.Show(ctx, r, task)
	if err != nil {
		return "", err
	}
	if bead.Parent == "" {
		return "", nil
	}
	return g.due(ctx, r, bead.Parent)
}

// GateEpic returns epic when its completed children make it due for review.
func (g *Gate) GateEpic(ctx context.Context, r repo.Repo, epic string) (string, error) {
	closed, err := g.ChildrenClosed(ctx, r, epic)
	if err != nil || !closed {
		return "", err
	}
	if !g.Enabled() {
		if err := g.beads.Close(ctx, r, epic, disabledCloseReason); err != nil {
			return "", err
		}
		return "", nil
	}
	return g.dueClosed(ctx, r, epic)
}

func (g *Gate) due(ctx context.Context, r repo.Repo, epic string) (string, error) {
	closed, err := g.ChildrenClosed(ctx, r, epic)
	if err != nil || !closed {
		return "", err
	}
	return g.dueClosed(ctx, r, epic)
}

func (g *Gate) dueClosed(ctx context.Context, r repo.Repo, epic string) (string, error) {
	k, ok := g.key(r, epic)
	if !ok || g.inFlight(r) {
		return "", nil
	}
	if g.attempts(k) >= g.MaxRetries() {
		if !g.exhaust(k) {
			_ = g.beads.Comment(ctx, r, epic, fmt.Sprintf(retryExhaustedComment, g.MaxRetries()))
		}
		return "", nil
	}
	if !g.NoteEpicLand(ctx, r, epic) {
		return "", nil
	}
	return epic, nil
}
