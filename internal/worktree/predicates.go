package worktree

import (
	"context"
	"fmt"
	"strings"
)

// SyncResult describes the outcome of a sync attempt.
type SyncResult uint8

const (
	SyncFailed SyncResult = iota
	SyncSynced
	SyncDirty
	SyncConflict
)

// String returns the stable name of a sync result.
func (r SyncResult) String() string {
	switch r {
	case SyncFailed:
		return "failed"
	case SyncSynced:
		return "synced"
	case SyncDirty:
		return "dirty"
	case SyncConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// dirty reports whether worktree status is unsafe to reset.
func (m *Manager) dirty(ctx context.Context, worktree string) bool {
	exit, output, err := m.gitAt(ctx, worktree, "status", "--porcelain")
	if err != nil || exit != 0 {
		m.warnf("worktree status failed (exit %d, output %q): %v", exit, output, err)
		return true
	}
	return strings.TrimSpace(output) != ""
}

// ancestor reports whether ancestor is reachable from descendant.
func (m *Manager) ancestor(ctx context.Context, repo Repo, ancestor, descendant string) (bool, error) {
	if strings.TrimSpace(ancestor) == "" || strings.TrimSpace(descendant) == "" {
		return false, ErrUnresolvedRepo
	}

	exit, output, err := m.git(ctx, repo, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil && exit == 0 {
		return true, nil
	}
	if err == nil && exit == 1 {
		return false, nil
	}

	failure := fmt.Errorf("%w: ancestor check (exit %d, output %q)", ErrSyncFailed, exit, output)
	if err != nil {
		failure = fmt.Errorf("%w: %w", failure, err)
	}
	m.warnf("%v", failure)
	return false, failure
}
