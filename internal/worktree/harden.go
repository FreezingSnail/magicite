package worktree

import (
	"context"
	"fmt"
	"strings"
)

// ConfigEntry names a repository configuration value required by seat worktrees.
type ConfigEntry struct {
	Key   string
	Value string
}

// HardenConfig returns the repository configuration required by seat worktrees.
func HardenConfig() []ConfigEntry {
	return []ConfigEntry{
		{Key: "merge.ff", Value: "only"},
		{Key: "pull.rebase", Value: "true"},
		{Key: "commit.cleanup", Value: "strip"},
	}
}

// Harden applies the seat worktree configuration to repo.
func (m *Manager) Harden(ctx context.Context, repo Repo) error {
	if _, err := m.Root(repo); err != nil {
		m.warnf("worktree hardening skipped: %v", err)
		return err
	}

	var failed []string
	for _, entry := range HardenConfig() {
		exit, output, err := m.git(ctx, repo, "config", entry.Key, entry.Value)
		if err == nil && exit == 0 {
			continue
		}
		m.warnf("worktree hardening failed for %s=%s (exit %d): %q: %v", entry.Key, entry.Value, exit, output, err)
		failed = append(failed, entry.Key)
	}
	if len(failed) != 0 {
		return fmt.Errorf("%w: %s", ErrHardenFailed, strings.Join(failed, ", "))
	}
	return nil
}
