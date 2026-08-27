package worktree

import (
	"context"
	"fmt"
	"os"
)

// Create provisions a worktree for seat and returns its registered path.
func (m *Manager) Create(ctx context.Context, repo Repo, seat string) (string, error) {
	root, err := m.Root(repo)
	if err != nil {
		return "", err
	}
	path, err := m.Path(repo, seat)
	if err != nil {
		return "", err
	}
	branch, err := m.Branch(repo, seat)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		failure := createFailure("workspace root", -1, "", err)
		m.warnf("seat %s creation: %v", seat, failure)
		return "", failure
	}
	if err := m.Harden(ctx, repo); err != nil {
		return "", err
	}

	exit, output, err := m.git(ctx, repo, "worktree", "add", path, "-b", branch)
	if err != nil {
		failure := createFailure("worktree add", exit, output, err)
		m.warnf("seat %s creation: %v", seat, failure)
		return "", failure
	}
	if exit != 0 {
		exit, output, err = m.git(ctx, repo, "worktree", "add", path, branch)
		if err != nil {
			failure := createFailure("worktree add", exit, output, err)
			m.warnf("seat %s creation: %v", seat, failure)
			return "", failure
		}
		if exit != 0 {
			failure := createFailure("worktree add", exit, output, nil)
			m.warnf("seat %s creation: %v", seat, failure)
			return "", failure
		}
	}

	entry, registered, err := m.Info(ctx, repo, path)
	if err != nil {
		return "", createFailure("worktree registration", -1, "", err)
	}
	if !registered {
		failure := fmt.Errorf("%w: seat %s is not registered", ErrCreateFailed, seat)
		m.warnf("seat %s creation: %v", seat, failure)
		return "", failure
	}
	return entry.Path, nil
}

func createFailure(step string, exit int, output string, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %s (exit %d, output %q): %w", ErrCreateFailed, step, exit, output, err)
	}
	return fmt.Errorf("%w: %s (exit %d, output %q)", ErrCreateFailed, step, exit, output)
}
