package dispatch

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

// RepoEpics pairs a repository with its open epics.
type RepoEpics struct {
	Repo  repo.Repo
	Epics []string
}

// DecomposeEpic starts one designer session for an undecomposed epic.
func (d *Dispatcher) DecomposeEpic(ctx context.Context, repository repo.Repo, epic string) string {
	if !repository.Valid() || d.isDraining() {
		return ""
	}

	active, cap := d.ActiveCount(Designer), d.RoleCap(Designer)
	if active >= cap {
		d.log(logging.Info, "decompose", map[string]any{
			"epic": epic, "repo": repository.LogName(), "result": "at-cap", "active": active, "seats": active, "cap": cap,
		})
		return ""
	}
	if d.FreeSeat(Designer) == "" {
		d.decomposeWarning(repository, epic, "no free designer seat")
		return ""
	}

	handle := d.Design(ctx, repository, epic)
	if handle == "" {
		if active = d.ActiveCount(Designer); active >= cap {
			d.log(logging.Info, "decompose", map[string]any{
				"epic": epic, "repo": repository.LogName(), "result": "at-cap", "active": active, "seats": active, "cap": cap,
			})
			return ""
		}
		if d.FreeSeat(Designer) == "" {
			d.decomposeWarning(repository, epic, "no free designer seat")
			return ""
		}
		d.decomposeWarning(repository, epic, "sync refusal or claim failure")
		return ""
	}
	d.MarkDecomposition(handle)
	d.log(logging.Info, "decompose", map[string]any{
		"epic": epic, "repo": repository.LogName(), "handle": handle,
	})
	return handle
}

func (d *Dispatcher) decomposeWarning(repository repo.Repo, epic, reason string) {
	d.log(logging.Warn, "decompose", map[string]any{
		"epic": epic, "repo": repository.LogName(), "reason": reason,
	})
}

// EpicPass dispatches undecomposed epics and gates fully closed epics. It
// returns false if any child query failed or the context was cancelled.
func (d *Dispatcher) EpicPass(ctx context.Context, repository repo.Repo, epics []string) bool {
	if d.isDraining() || ctx.Err() != nil {
		return false
	}

	children, complete := fanOut(ctx, uniqueEpics(epics), func(ctx context.Context, epic string) ([]string, error) {
		return d.beads.EpicChildren(ctx, repository, epic)
	})
	if !complete {
		return false
	}

	allSucceeded := true
	decomposed := make([]string, 0, len(children))
	for _, result := range children {
		if result.err != nil {
			allSucceeded = false
			d.RepoWarn(repository, result.err.Error())
			continue
		}
		d.RepoOK(repository)
		if len(result.Value) == 0 {
			d.DecomposeEpic(ctx, repository, result.Item)
			continue
		}
		decomposed = append(decomposed, result.Item)
	}
	if ctx.Err() != nil {
		return false
	}

	openChildren, complete := fanOut(ctx, decomposed, func(ctx context.Context, epic string) ([]string, error) {
		return d.beads.EpicOpenChildren(ctx, repository, epic)
	})
	if !complete {
		return false
	}
	for _, result := range openChildren {
		if result.err != nil {
			allSucceeded = false
			d.RepoWarn(repository, result.err.Error())
			continue
		}
		d.RepoOK(repository)
	}
	if !allSucceeded || d.isDraining() || ctx.Err() != nil {
		return allSucceeded && ctx.Err() == nil && !d.isDraining()
	}
	for _, result := range openChildren {
		if len(result.Value) == 0 {
			_, _ = d.gate.GateEpic(ctx, repository, result.Item)
		}
	}
	return true
}

// EpicPasses concurrently runs one epic pass for each repository.
func (d *Dispatcher) EpicPasses(ctx context.Context, repoEpics []RepoEpics) bool {
	if d.isDraining() || ctx.Err() != nil {
		return false
	}
	results := FanOut(ctx, repoEpics, func(ctx context.Context, group RepoEpics) (bool, error) {
		return d.EpicPass(ctx, group.Repo, group.Epics), nil
	})
	if results == nil && len(repoEpics) != 0 {
		return false
	}
	for _, result := range results {
		if !result.Value {
			return false
		}
	}
	return true
}

func uniqueEpics(epics []string) []string {
	seen := make(map[string]struct{}, len(epics))
	unique := make([]string, 0, len(epics))
	for _, epic := range epics {
		if _, duplicate := seen[epic]; duplicate {
			continue
		}
		seen[epic] = struct{}{}
		unique = append(unique, epic)
	}
	return unique
}

func (d *Dispatcher) isDraining() bool {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.draining
}
