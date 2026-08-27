// Package worktreetest provides an in-memory worktree.Provisioner for consumer tests.
package worktreetest

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/FreezingSnail/magicite/internal/worktree"
)

// Seat scripts results for one seat.
type Seat struct {
	Path       string
	Result     worktree.SyncResult
	EnsureErr  error
	SyncErr    error
	CleanupErr error
}

// Call records one Provisioner method invocation.
type Call struct {
	Op   string
	Repo string
	Seat string
}

// Fake is a synchronized in-memory worktree.Provisioner.
type Fake struct {
	WorkspacePath string

	mu    sync.Mutex
	seats map[string]Seat
	calls []Call
}

var _ worktree.Provisioner = (*Fake)(nil)

// New creates a fake with scripted seat results.
func New(seats map[string]Seat) *Fake {
	copied := make(map[string]Seat, len(seats))
	for name, seat := range seats {
		copied[name] = seat
	}
	return &Fake{seats: copied}
}

// Calls returns an independent copy of recorded calls.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// Reset clears recorded calls.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

// Path resolves a seat location without touching the filesystem.
func (f *Fake) Path(repo worktree.Repo, seat string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("path", repo, seat)
	return f.path(repo, seat)
}

// Branch returns the branch name reserved for seat.
func (f *Fake) Branch(repo worktree.Repo, seat string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("branch", repo, seat)
	if _, err := f.path(repo, seat); err != nil {
		return "", err
	}
	return seat, nil
}

// Ensure returns the scripted seat path and error.
func (f *Fake) Ensure(_ context.Context, repo worktree.Repo, seat string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ensure", repo, seat)
	path, err := f.path(repo, seat)
	if err != nil {
		return "", err
	}
	if scripted, ok := f.seats[seat]; ok {
		return scripted.Path, scripted.EnsureErr
	}
	return path, nil
}

// Sync returns the scripted seat result and error.
func (f *Fake) Sync(_ context.Context, repo worktree.Repo, seat string) (worktree.SyncResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("sync", repo, seat)
	if _, err := f.path(repo, seat); err != nil {
		return worktree.SyncFailed, err
	}
	if scripted, ok := f.seats[seat]; ok {
		return scripted.Result, scripted.SyncErr
	}
	return worktree.SyncSynced, nil
}

// Cleanup returns the scripted cleanup error.
func (f *Fake) Cleanup(_ context.Context, repo worktree.Repo, seat string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("cleanup", repo, seat)
	if _, err := f.path(repo, seat); err != nil {
		return err
	}
	if scripted, ok := f.seats[seat]; ok {
		return scripted.CleanupErr
	}
	return nil
}

func (f *Fake) path(repo worktree.Repo, seat string) (string, error) {
	if !validSeat(seat) {
		return "", worktree.ErrInvalidSeat
	}
	if repo == nil || repo.Name() == "" || strings.TrimSpace(repo.Root()) == "" || repo.Integration() == "" {
		return "", worktree.ErrUnresolvedRepo
	}
	workspace := f.WorkspacePath
	if workspace == "" {
		workspace = "harness/workspaces"
	}
	return filepath.Join(repo.Root(), workspace, seat), nil
}

func (f *Fake) record(op string, repo worktree.Repo, seat string) {
	name := ""
	if repo != nil {
		name = repo.Name()
	}
	f.calls = append(f.calls, Call{Op: op, Repo: name, Seat: seat})
}

func validSeat(seat string) bool {
	return strings.TrimSpace(seat) != "" && seat != "." && seat != ".."
}
