package worktree

import (
	"context"
	"fmt"
	"os"
)

// Ensure returns the registered seat worktree or creates it when absent.
func (m *Manager) Ensure(ctx context.Context, repo Repo, seat string) (string, error) {
	path, err := m.Path(repo, seat)
	if err != nil {
		return "", err
	}

	registered, err := m.Registered(ctx, repo, path)
	if err != nil {
		return "", err
	}
	if registered {
		entry, _, err := m.Info(ctx, repo, path)
		if err != nil {
			return "", err
		}
		return entry.Path, nil
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return m.Create(ctx, repo, seat)
	}
	if err != nil {
		return m.occupied(path, err)
	}
	if !info.IsDir() {
		return m.occupied(path, nil)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return m.occupied(path, err)
	}
	if len(entries) != 0 {
		return m.occupied(path, nil)
	}
	if err := os.Remove(path); err != nil {
		return m.occupied(path, err)
	}
	return m.Create(ctx, repo, seat)
}

func (m *Manager) occupied(path string, cause error) (string, error) {
	failure := fmt.Errorf("%w: %s", ErrOccupiedPath, path)
	if cause != nil {
		failure = fmt.Errorf("%w: %w", failure, cause)
	}
	m.warnf("%v", failure)
	return "", failure
}
