package worktree

import (
	"context"
	"fmt"
	"os"
)

// Cleanup removes the worktree and branch reserved for seat.
func (m *Manager) Cleanup(ctx context.Context, repo Repo, seat string) error {
	path, err := m.Path(repo, seat)
	if err != nil {
		return err
	}
	branch, err := m.Branch(repo, seat)
	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil
	}

	exit, output, err := m.git(ctx, repo, "rev-parse", "--verify", branch)
	if err != nil {
		failure := cleanupFailure("branch verification", exit, output, err)
		m.warnf("seat %s cleanup: %v", seat, failure)
		return failure
	}
	if exit != 0 {
		m.warnf("seat %s cleanup: branch %s absent (exit %d): %q", seat, branch, exit, output)
		return nil
	}

	exit, output, err = m.git(ctx, repo, "worktree", "remove", "--force", path)
	if err != nil || exit != 0 {
		failure := cleanupFailure("worktree removal", exit, output, err)
		m.warnf("seat %s cleanup: %v", seat, failure)
		return failure
	}

	exit, output, err = m.git(ctx, repo, "branch", "-D", branch)
	if err != nil || exit != 0 {
		failure := cleanupFailure("branch removal", exit, output, err)
		m.warnf("seat %s cleanup: %v", seat, failure)
		return failure
	}

	exit, output, err = m.git(ctx, repo, "worktree", "prune")
	if err != nil || exit != 0 {
		m.warnf("seat %s cleanup: prune failed (exit %d): %q: %v", seat, exit, output, err)
	}
	return nil
}

func cleanupFailure(step string, exit int, output string, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %s (exit %d, output %q): %w", ErrCleanupFailed, step, exit, output, err)
	}
	return fmt.Errorf("%w: %s (exit %d, output %q)", ErrCleanupFailed, step, exit, output)
}
