package dispatch

import (
	"context"
	"errors"
	"sync"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

// spawnMu preserves seat exclusivity across each dispatch admission sequence.
var spawnMu sync.Mutex

// Implement dispatches one task to an available implementer seat.
func (d *Dispatcher) Implement(ctx context.Context, repository repo.Repo, task string) string {
	return d.spawn(ctx, repository, task, Implementer, "", "", "")
}

// Design dispatches one task to an available designer seat.
func (d *Dispatcher) Design(ctx context.Context, repository repo.Repo, task string) string {
	return d.spawn(ctx, repository, task, Designer, "", "", "")
}

// Repair dispatches one task to its named repair seat.
func (d *Dispatcher) Repair(ctx context.Context, repository repo.Repo, seat, task string) string {
	handle := d.spawn(ctx, repository, task, Repairer, seat, "", "")
	if handle != "" {
		d.SetStatus(handle, Repairing)
	}
	return handle
}

// Review dispatches an epic review on the reviewer seat.
func (d *Dispatcher) Review(ctx context.Context, repository repo.Repo, epic string) string {
	run, err := d.gate.ReviewPlan(ctx, repository, epic)
	if err != nil {
		d.log(logging.Warn, "dispatch-refused", dispatchFields(repository, epic, Reviewer, ""))
		return ""
	}
	handle := d.spawn(ctx, repository, epic, Reviewer, "", run.Model, run.Plan)
	if handle != "" {
		d.gate.NoteSession(handle, repository, epic)
	}
	return handle
}

// SeatReady ensures a seat exists and is synchronized before it receives work.
func (d *Dispatcher) SeatReady(ctx context.Context, repository repo.Repo, role Role, seat, task string) bool {
	if _, err := d.workspaces.Ensure(ctx, repository, seat); err != nil {
		d.log(logging.Warn, "seat-refused", dispatchFields(repository, task, role, seat))
		return false
	}
	if role == Repairer {
		return true
	}
	result, err := d.workspaces.Sync(ctx, repository, seat)
	if err != nil {
		d.log(logging.Warn, "seat-refused", dispatchFields(repository, task, role, seat))
		return false
	}
	if result != SyncConflict {
		return true
	}
	d.log(logging.Warn, "seat-refused", dispatchFields(repository, task, role, seat))
	_ = d.beads.Comment(ctx, repository, task, "seat "+seat+" holds unlanded work conflicting with main.")
	return false
}

func (d *Dispatcher) spawn(ctx context.Context, repository repo.Repo, task string, role Role, seat, model, plan string) string {
	spawnMu.Lock()
	defer spawnMu.Unlock()

	if !repository.Valid() {
		d.log(logging.Warn, "dispatch-refused", dispatchFields(repository, task, role, seat))
		return ""
	}
	humanOnly, err := d.beads.HumanOnly(ctx, repository, task)
	if err != nil {
		d.log(logging.Warn, "dispatch-refused", dispatchFields(repository, task, role, seat))
		return ""
	}
	if humanOnly {
		d.log(logging.Info, "human-hold", dispatchFields(repository, task, role, seat))
		return ""
	}
	if d.ActiveCount(role) >= d.RoleCap(role) {
		return ""
	}
	if seat == "" {
		seat = d.FreeSeat(role)
	}
	if seat == "" || !d.SeatReady(ctx, repository, role, seat, task) {
		return ""
	}
	if role != Reviewer {
		if err := d.beads.Claim(ctx, repository, task); err != nil {
			d.log(logging.Warn, "claim-failed", dispatchFields(repository, task, role, seat))
			return ""
		}
	}
	return d.spawnSession(ctx, repository, task, role, seat, model, plan)
}

func (d *Dispatcher) spawnSession(ctx context.Context, repository repo.Repo, task string, role Role, seat, model, plan string) string {
	difficulty := config.DifficultyHigh
	if role == Implementer && model == "" {
		var err error
		difficulty, err = d.beads.Difficulty(ctx, repository, task)
		if err != nil {
			d.dispatchFailed(ctx, repository, task, role, seat, err)
			return ""
		}
	}
	resolution, err := config.Resolve(d.config, string(role), difficulty)
	if err != nil {
		d.dispatchFailed(ctx, repository, task, role, seat, err)
		return ""
	}
	if model != "" {
		resolution.Model = model
	}
	if plan == "" {
		plan, err = d.PlanFor(ctx, repository, role, task, seat)
		if err != nil {
			d.dispatchFailed(ctx, repository, task, role, seat, err)
			return ""
		}
	}
	workdir := repository.Root
	if role != Reviewer {
		workdir, err = d.workspaces.Path(repository, seat)
		if err != nil {
			d.dispatchFailed(ctx, repository, task, role, seat, err)
			return ""
		}
	}
	handle, err := d.runner.Run(ctx, RunSpec{
		Workdir: workdir,
		Backend: resolution.Backend,
		Model:   resolution.Model,
		Agent:   resolution.Agent,
		Effort:  resolution.Effort,
		Plan:    plan,
	})
	if err != nil || handle == "" {
		if err == nil {
			err = errors.New("dispatch: runner returned empty handle")
		}
		d.dispatchFailed(ctx, repository, task, role, seat, err)
		return ""
	}
	d.Add(Session{
		Handle: handle, Repo: repository, Task: task, Role: role, Seat: seat,
		Backend: resolution.Backend, Model: resolution.Model, Difficulty: difficulty,
		Effort: resolution.Effort, Agent: resolution.Agent, Status: Working,
	})
	fields := dispatchFields(repository, task, role, seat)
	fields["backend"] = resolution.Backend
	fields["model"] = resolution.Model
	fields["difficulty"] = difficulty
	fields["effort"] = resolution.Effort
	fields["handle"] = handle
	d.log(logging.Info, logging.KindPickup, fields)
	return handle
}

func (d *Dispatcher) dispatchFailed(ctx context.Context, repository repo.Repo, task string, role Role, seat string, err error) {
	fields := dispatchFields(repository, task, role, seat)
	fields["error"] = err.Error()
	d.log(logging.Error, "dispatch-failed", fields)
	_ = d.beads.Comment(ctx, repository, task, "session failed; task is left open.")
	if role != Reviewer {
		_ = d.beads.Release(ctx, repository, task)
	}
}

func dispatchFields(repository repo.Repo, task string, role Role, seat string) map[string]any {
	return map[string]any{"task": task, "repo": repository.LogName(), "role": role, "seat": seat}
}
