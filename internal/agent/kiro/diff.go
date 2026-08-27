package kiro

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/FreezingSnail/magicite/internal/agent"
	executil "github.com/FreezingSnail/magicite/internal/exec"
)

const (
	modified  = "modified"
	staged    = "staged"
	untracked = "untracked"
)

// Diff returns Git changes in the worktree recorded for handle.
func (a *Adapter) Diff(ctx context.Context, handle agent.Handle) ([]agent.FileDiff, error) {
	state, ok := a.store.Get(handle)
	if !ok {
		return nil, fmt.Errorf("%w: %s", agent.ErrUnknownHandle, handle)
	}

	paths := make(map[string]string)
	for _, source := range []struct {
		args   []string
		status string
		zero   bool
	}{
		{args: []string{"diff", "--name-only"}, status: modified},
		{args: []string{"diff", "--cached", "--name-only"}, status: staged},
		{args: []string{"ls-files", "--others", "--exclude-standard", "-z"}, status: untracked, zero: true},
	} {
		output, err := git(ctx, state.workdir, source.args, false)
		if err != nil {
			return nil, err
		}
		for _, path := range splitPaths(output, source.zero) {
			if err := withinWorktree(state.workdir, path); err != nil {
				return nil, err
			}
			if _, exists := paths[path]; !exists {
				paths[path] = source.status
			}
		}
	}

	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	diffs := make([]agent.FileDiff, 0, len(names))
	for _, path := range names {
		patch, additions, deletions, err := fileDiff(ctx, state.workdir, path, paths[path])
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, agent.FileDiff{
			File: path, Patch: patch, Status: paths[path], Additions: additions, Deletions: deletions,
		})
	}
	return diffs, nil
}

func splitPaths(output string, zero bool) []string {
	separator := "\n"
	if zero {
		separator = "\x00"
	}
	var paths []string
	for _, path := range strings.Split(output, separator) {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func withinWorktree(workdir, path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("Git path outside worktree: %q", path)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Git path outside worktree: %q", path)
	}
	absolute := filepath.Join(workdir, clean)
	relative, err := filepath.Rel(workdir, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Git path outside worktree: %q", path)
	}
	return nil
}

func fileDiff(ctx context.Context, workdir, path, status string) (string, int, int, error) {
	var patchArgs, statArgs []string
	switch status {
	case modified:
		patchArgs = []string{"diff", "--no-ext-diff", "--", path}
		statArgs = []string{"diff", "--numstat", "--", path}
	case staged:
		patchArgs = []string{"diff", "--cached", "--no-ext-diff", "--", path}
		statArgs = []string{"diff", "--cached", "--numstat", "--", path}
	case untracked:
		patchArgs = []string{"diff", "--no-index", "--", "/dev/null", path}
		statArgs = []string{"diff", "--no-index", "--numstat", "--", "/dev/null", path}
	default:
		return "", 0, 0, fmt.Errorf("unknown Git diff status %q", status)
	}
	patch, err := git(ctx, workdir, patchArgs, status == untracked)
	if err != nil {
		return "", 0, 0, err
	}
	numstat, err := git(ctx, workdir, statArgs, status == untracked)
	if err != nil {
		return "", 0, 0, err
	}
	additions, deletions, err := parseNumstat(numstat)
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse Git numstat for %q: %w", path, err)
	}
	return patch, additions, deletions, nil
}

func git(ctx context.Context, workdir string, args []string, allowDifference bool) (string, error) {
	stdout, stderr, code, runErr := executil.Run(ctx, workdir, "git", args...)
	if runErr != nil && !(allowDifference && code == 1) {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), runErr, strings.TrimSpace(string(stderr)))
	}
	return string(stdout), nil
}

func parseNumstat(output string) (int, int, error) {
	line := strings.TrimSpace(output)
	if line == "" {
		return 0, 0, nil
	}
	fields := strings.SplitN(line, "\t", 3)
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("invalid numstat %q", line)
	}
	additions, err := numstatCount(fields[0])
	if err != nil {
		return 0, 0, err
	}
	deletions, err := numstatCount(fields[1])
	if err != nil {
		return 0, 0, err
	}
	return additions, deletions, nil
}

func numstatCount(value string) (int, error) {
	if value == "-" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
