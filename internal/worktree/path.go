package worktree

import (
	"os"
	"path/filepath"
	"strings"
)

// Root returns the absolute workspace root for repo without creating it.
func (m *Manager) Root(repo Repo) (string, error) {
	if repo == nil || repo.Name() == "" || repo.Integration() == "" {
		return "", ErrUnresolvedRepo
	}

	info, err := os.Stat(repo.Root())
	if err != nil || !info.IsDir() {
		return "", ErrUnresolvedRepo
	}
	root, err := filepath.Abs(repo.Root())
	if err != nil {
		return "", ErrUnresolvedRepo
	}
	root = filepath.Clean(root)
	workspace := filepath.Clean(filepath.Join(root, m.workspacePath))
	if !within(root, workspace) {
		return "", ErrEscapingPath
	}
	return workspace, nil
}

// Path returns the absolute path reserved for seat without creating it.
func (m *Manager) Path(repo Repo, seat string) (string, error) {
	if !validSeat(seat) {
		return "", ErrInvalidSeat
	}
	root, err := m.Root(repo)
	if err != nil {
		return "", err
	}
	path := filepath.Clean(filepath.Join(root, seat))
	if !within(root, path) {
		return "", ErrEscapingPath
	}
	return path, nil
}

// Branch returns the branch name reserved for seat.
func (m *Manager) Branch(repo Repo, seat string) (string, error) {
	if !validSeat(seat) {
		return "", ErrInvalidSeat
	}
	if _, err := m.Root(repo); err != nil {
		return "", err
	}
	if seat == repo.Integration() {
		return "", ErrProtectedBranch
	}
	return seat, nil
}

func validSeat(seat string) bool {
	return strings.TrimSpace(seat) != "" && seat != "." && seat != ".." &&
		!strings.ContainsAny(seat, "/\\")
}

func escapesRoot(path string) bool {
	path = filepath.Clean(path)
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && !escapesRoot(rel)
}
