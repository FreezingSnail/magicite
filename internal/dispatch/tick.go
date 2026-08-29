package dispatch

import (
	"context"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

const repoWarnInterval = 300 * time.Second

// RepoWarn records a rate-limited repository skip warning.
func (d *Dispatcher) RepoWarn(repository repo.Repo, reason string) {
	name := repository.LogName()
	now := d.clock.Now()
	d.stateMu.Lock()
	last, warned := d.repoWarnedAt[name]
	if warned && now.Sub(last) < repoWarnInterval {
		d.stateMu.Unlock()
		return
	}
	d.repoWarnedAt[name] = now
	d.stateMu.Unlock()
	d.log(logging.Warn, "repo-skip", map[string]any{"repo": name, "reason": reason})
}

// RepoOK clears a repository's skip-warning latch after a successful query.
func (d *Dispatcher) RepoOK(repository repo.Repo) {
	d.stateMu.Lock()
	delete(d.repoWarnedAt, repository.LogName())
	d.stateMu.Unlock()
}

// TickInFlight reports whether a tick currently owns dispatch work.
func (d *Dispatcher) TickInFlight() bool {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.tickInFlight
}

// Tick queries every registered repository and dispatches eligible rework and
// ready tasks. It never lets one repository's query failure block another.
func (d *Dispatcher) Tick(ctx context.Context) {
	d.stateMu.Lock()
	reason := ""
	switch {
	case d.draining:
		reason = "draining"
	case d.tickInFlight:
		reason = "in-flight"
	default:
		d.tickInFlight = true
	}
	d.stateMu.Unlock()
	if reason != "" {
		d.log(logging.Debug, "tick-skipped", map[string]any{"reason": reason})
		return
	}
	defer func() {
		d.stateMu.Lock()
		d.tickInFlight = false
		d.stateMu.Unlock()
	}()

	repositories := d.repos.List(ctx)
	if len(repositories) == 0 {
		d.RepoWarn(repo.Repo{}, "empty-registry")
		return
	}
	if ctx.Err() != nil {
		return
	}

	drift, complete := fanOut(ctx, repositories, d.beads.DriftFixTasks)
	if !complete {
		return
	}
	driftTasks := make(map[repo.Repo][]string, len(repositories))
	driftFailed := make(map[repo.Repo]bool)
	for _, result := range drift {
		if result.err != nil {
			driftFailed[result.Item] = true
			d.RepoWarn(result.Item, result.err.Error())
			continue
		}
		d.RepoOK(result.Item)
		driftTasks[result.Item] = result.Value
	}

	inProgress, complete := fanOut(ctx, repositories, d.beads.InProgress)
	if !complete {
		return
	}
	rework := 0
	for _, result := range inProgress {
		if ctx.Err() != nil {
			return
		}
		if result.err != nil {
			d.RepoWarn(result.Item, result.err.Error())
			continue
		}
		d.RepoOK(result.Item)
		if driftFailed[result.Item] {
			continue
		}
		rework += d.RecoverTasks(ctx, result.Item, result.Value, driftTasks[result.Item])
	}

	ready, complete := fanOut(ctx, repositories, d.beads.Ready)
	if !complete || ctx.Err() != nil {
		return
	}
	readyByRepo := make(map[repo.Repo][]ReadyEntry, len(repositories))
	readyRepos := make([]repo.Repo, 0, len(repositories))
	for _, result := range ready {
		if result.err != nil {
			d.RepoWarn(result.Item, result.err.Error())
			continue
		}
		d.RepoOK(result.Item)
		readyRepos = append(readyRepos, result.Item)
		for _, entry := range result.Value {
			if normalized, ok := NormalizeReady(result.Item, entry); ok {
				readyByRepo[result.Item] = append(readyByRepo[result.Item], normalized)
			}
		}
	}

	groups := make([]RepoReady, 0, len(repositories))
	held := 0
	for _, repository := range repositories {
		if ctx.Err() != nil {
			return
		}
		entries := readyByRepo[repository]
		isHeld, reason := d.hold(ctx, repository)
		if driftFailed[repository] {
			isHeld = true
			reason = "drift-fix unavailable"
		}
		if isHeld {
			held += len(entries)
			if len(entries) != 0 {
				d.log(logging.Info, "fleet-hold", map[string]any{"repo": repository.LogName(), "reason": reason, "held": len(entries)})
			}
			continue
		}
		groups = append(groups, RepoReady{Repo: repository, Entries: entries})
	}

	merged := MergeReady(groups)
	for _, entry := range merged {
		if ctx.Err() != nil {
			return
		}
		d.Implement(ctx, entry.Repo, entry.Task)
	}

	openEpics, complete := fanOut(ctx, readyRepos, d.beads.OpenEpics)
	if !complete || ctx.Err() != nil {
		return
	}
	repoEpics := make([]RepoEpics, 0, len(openEpics))
	for _, result := range openEpics {
		if result.err != nil {
			d.RepoWarn(result.Item, result.err.Error())
			continue
		}
		d.RepoOK(result.Item)
		repoEpics = append(repoEpics, RepoEpics{Repo: result.Item, Epics: result.Value})
	}
	d.EpicPasses(ctx, repoEpics)

	d.log(logging.Debug, "tick", map[string]any{
		"repos": len(repositories), "ready": len(merged), "rework": rework,
		"held": held, "sessions": len(d.Sessions()),
	})
}

func (d *Dispatcher) hold(ctx context.Context, repository repo.Repo) (held bool, reason string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			held = true
			reason = "gate panicked"
			d.RepoWarn(repository, reason)
		}
	}()
	held, err := d.gate.Hold(ctx, repository)
	if err != nil {
		d.RepoWarn(repository, err.Error())
		return true, err.Error()
	}
	if held {
		return true, "review hold"
	}
	return false, ""
}
