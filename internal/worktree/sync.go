package worktree

import (
	"context"
	"fmt"
	"strings"
)

// Sync moves a registered seat worktree onto its repository integration base.
func (m *Manager) Sync(ctx context.Context, repo Repo, seat string) (SyncResult, error) {
	path, err := m.Path(repo, seat)
	if err != nil {
		return SyncFailed, err
	}
	branch, err := m.Branch(repo, seat)
	if err != nil {
		return SyncFailed, err
	}

	registered, err := m.Registered(ctx, repo, path)
	if err != nil {
		return SyncFailed, fmt.Errorf("%w: %w", ErrMissingWorktree, err)
	}
	if !registered {
		failure := fmt.Errorf("%w: repo %s seat %s at %s", ErrMissingWorktree, repo.Name(), seat, path)
		m.warnf("%v", failure)
		return SyncFailed, failure
	}

	integration := repo.Integration()
	ahead, err := m.ancestor(ctx, repo, integration, branch)
	if err != nil {
		return SyncFailed, err
	}
	if ahead {
		return SyncSynced, nil
	}

	if m.dirty(ctx, path) {
		m.warnf("repo %s seat %s stays on a stale base: worktree is dirty", repo.Name(), seat)
		return SyncDirty, nil
	}

	unchanged, err := m.ancestor(ctx, repo, branch, integration)
	if err != nil {
		return SyncFailed, err
	}
	if unchanged {
		exit, output, runErr := m.gitAt(ctx, path, "reset", "--hard", integration)
		if runErr == nil && exit == 0 {
			return SyncSynced, nil
		}
		failure := syncFailure("reset", exit, output, runErr)
		m.warnf("repo %s seat %s sync: %v", repo.Name(), seat, failure)
		return SyncFailed, failure
	}

	exit, output, runErr := m.gitAt(ctx, path, "rebase", integration, branch)
	if runErr == nil && exit == 0 {
		return SyncSynced, nil
	}

	m.abortRebase(ctx, path, repo, seat)
	if strings.Contains(strings.ToLower(output), "conflict") {
		m.warnf("repo %s seat %s stays on a stale base: rebase conflict", repo.Name(), seat)
		return SyncConflict, nil
	}

	failure := syncFailure("rebase", exit, output, runErr)
	m.warnf("repo %s seat %s sync: %v", repo.Name(), seat, failure)
	return SyncFailed, failure
}

func (m *Manager) abortRebase(ctx context.Context, path string, repo Repo, seat string) {
	exit, output, err := m.gitAt(ctx, path, "rebase", "--abort")
	if err != nil || exit != 0 {
		m.warnf("repo %s seat %s rebase abort: %v", repo.Name(), seat, syncFailure("rebase abort", exit, output, err))
	}
}

func syncFailure(operation string, exit int, output string, err error) error {
	failure := fmt.Errorf("%w: %s (exit %d, output %q)", ErrSyncFailed, operation, exit, output)
	if err != nil {
		return fmt.Errorf("%w: %w", failure, err)
	}
	return failure
}
