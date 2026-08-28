package dispatch

import (
	"context"
	"fmt"

	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
)

const orphanedInProgressReason = "orphaned in_progress"

// RecoverError identifies a failure while loading a repository's in-progress tasks.
type RecoverError struct {
	Repo repo.Repo
	Err  error
}

func (e *RecoverError) Error() string {
	return fmt.Sprintf("dispatch: recover %s: %v", e.Repo.LogName(), e.Err)
}

func (e *RecoverError) Unwrap() error { return e.Err }

// Orphans returns in-progress tasks without a live session, preserving input order.
func (d *Dispatcher) Orphans(inProgress []string) []string {
	live := make(map[string]struct{})
	for _, session := range d.Sessions() {
		live[session.Task] = struct{}{}
	}

	orphans := make([]string, 0, len(inProgress))
	for _, task := range inProgress {
		if _, ok := live[task]; !ok {
			orphans = append(orphans, task)
		}
	}
	return orphans
}

// RecoverTasks re-dispatches eligible orphaned in-progress tasks.
func (d *Dispatcher) RecoverTasks(ctx context.Context, repository repo.Repo, inProgress, only []string) int {
	allow := make(map[string]struct{}, len(only))
	for _, task := range only {
		allow[task] = struct{}{}
	}

	attempted := make(map[string]struct{})
	dispatched := 0
	for _, task := range d.Orphans(inProgress) {
		if _, ok := attempted[task]; ok {
			continue
		}
		attempted[task] = struct{}{}
		if len(only) != 0 {
			if _, ok := allow[task]; !ok {
				continue
			}
		}
		if d.Implement(ctx, repository, task) == "" {
			continue
		}
		d.log(logging.Info, logging.KindRecovery, map[string]any{
			"task":   task,
			"repo":   repository.LogName(),
			"reason": orphanedInProgressReason,
		})
		dispatched++
	}
	return dispatched
}

// RecoverRepo loads and re-dispatches a repository's orphaned in-progress tasks.
func (d *Dispatcher) RecoverRepo(ctx context.Context, repository repo.Repo) (int, error) {
	inProgress, err := d.beads.InProgress(ctx, repository)
	if err != nil {
		return 0, &RecoverError{Repo: repository, Err: err}
	}
	return d.RecoverTasks(ctx, repository, inProgress, nil), nil
}
