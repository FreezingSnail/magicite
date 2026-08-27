package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Entry describes a worktree registered by git.
type Entry struct {
	Path   string
	Branch string
	Name   string
}

// ParseList parses the output of git worktree list --porcelain.
func ParseList(output string) ([]Entry, error) {
	entries := make([]Entry, 0)
	var entry Entry
	inRecord := false
	orphanedAttribute := false

	finish := func() error {
		if !inRecord {
			if orphanedAttribute {
				return ErrMalformedList
			}
			return nil
		}
		if entry.Path == "" {
			return ErrMalformedList
		}
		entry.Name = filepath.Base(filepath.Clean(entry.Path))
		entries = append(entries, entry)
		entry = Entry{}
		inRecord = false
		orphanedAttribute = false
		return nil
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			if err := finish(); err != nil {
				return nil, err
			}
			continue
		}
		if line == "worktree" || strings.HasPrefix(line, "worktree ") {
			if err := finish(); err != nil {
				return nil, err
			}
			entry.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree"))
			if entry.Path == "" {
				return nil, ErrMalformedList
			}
			inRecord = true
			continue
		}
		if !inRecord {
			orphanedAttribute = true
			continue
		}
		if branch, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			entry.Branch = branch
			continue
		}
		entry.Branch = ""
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return entries, nil
}

// List returns the worktrees registered by git for repo.
func (m *Manager) List(ctx context.Context, repo Repo) ([]Entry, error) {
	if repo == nil {
		return nil, ErrUnresolvedRepo
	}
	exit, output, err := m.git(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		failure := fmt.Errorf("git worktree list (exit %d, output %q): %w", exit, output, err)
		m.warnf("%v", failure)
		return nil, failure
	}
	if exit != 0 {
		failure := fmt.Errorf("git worktree list failed (exit %d, output %q)", exit, output)
		m.warnf("%v", failure)
		return nil, failure
	}
	return ParseList(output)
}

// Info returns the registered worktree whose path identifies dir.
func (m *Manager) Info(ctx context.Context, repo Repo, dir string) (Entry, bool, error) {
	if repo == nil {
		return Entry{}, false, ErrUnresolvedRepo
	}
	canonicalDir, err := canonicalPath(dir)
	if err != nil {
		return Entry{}, false, err
	}
	entries, err := m.List(ctx, repo)
	if err != nil {
		return Entry{}, false, err
	}
	for _, entry := range entries {
		path, err := canonicalPath(entry.Path)
		if err != nil {
			return Entry{}, false, err
		}
		if path == canonicalDir {
			return entry, true, nil
		}
	}
	return Entry{}, false, nil
}

// Registered reports whether dir is registered as a worktree for repo.
func (m *Manager) Registered(ctx context.Context, repo Repo, dir string) (bool, error) {
	_, registered, err := m.Info(ctx, repo, dir)
	return registered, err
}

func canonicalPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("resolve worktree path: empty path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve worktree path %q: %w", path, err)
	}
	absolute = filepath.Clean(absolute)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if os.IsNotExist(err) {
		return absolute, nil
	}
	return "", fmt.Errorf("resolve worktree path %q: %w", path, err)
}
