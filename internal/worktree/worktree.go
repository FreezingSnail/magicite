// Package worktree resolves per-seat worktree locations and runs git commands.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/FreezingSnail/magicite/internal/exec"
	"github.com/FreezingSnail/magicite/internal/logging"
)

const defaultWorkspacePath = "harness/workspaces"

var (
	ErrInvalidOptions  = errors.New("invalid worktree options")
	ErrUnresolvedRepo  = errors.New("unresolved repository")
	ErrInvalidSeat     = errors.New("invalid seat")
	ErrEscapingPath    = errors.New("escaping worktree path")
	ErrProtectedBranch = errors.New("protected integration branch")
	ErrMalformedList   = errors.New("malformed worktree list")
	ErrHardenFailed    = errors.New("worktree hardening failed")
	ErrCreateFailed    = errors.New("worktree creation failed")
	ErrOccupiedPath    = errors.New("occupied worktree path")
	ErrMissingWorktree = errors.New("missing worktree")
	ErrSyncFailed      = errors.New("worktree sync failed")
	ErrCleanupFailed   = errors.New("worktree cleanup failed")
)

// Repo supplies the repository data required by worktree operations.
type Repo interface {
	Name() string
	Root() string
	Integration() string
}

// Runner runs git with an explicit working directory and argv.
type Runner interface {
	Git(ctx context.Context, dir string, args ...string) (int, string, error)
}

// Options configures a Manager.
type Options struct {
	WorkspacePath string
	Runner        Runner
	Log           func(level logging.Level, kind string, fields map[string]any)
}

// Manager resolves worktree locations and supplies the shared git seam.
type Manager struct {
	workspacePath string
	runner        Runner
	log           func(level logging.Level, kind string, fields map[string]any)
}

// New builds a Manager with safe defaults.
func New(opts Options) (*Manager, error) {
	workspacePath := opts.WorkspacePath
	if workspacePath == "" {
		workspacePath = defaultWorkspacePath
	}
	workspacePath = filepath.Clean(workspacePath)
	if strings.TrimSpace(workspacePath) == "" || filepath.IsAbs(workspacePath) || escapesRoot(workspacePath) {
		return nil, ErrInvalidOptions
	}

	runner := opts.Runner
	if runner == nil {
		runner = ExecRunner()
	}
	log := opts.Log
	if log == nil {
		log = logging.Event
	}
	return &Manager{workspacePath: workspacePath, runner: runner, log: log}, nil
}

// ExecRunner returns a Runner backed by the process executor.
func ExecRunner() Runner { return execRunner{} }

type execRunner struct{}

func (execRunner) Git(ctx context.Context, dir string, args ...string) (int, string, error) {
	stdout, stderr, exitCode, runErr := exec.Run(ctx, dir, "git", args...)
	output := string(stdout) + string(stderr)
	if runErr == nil {
		return exitCode, output, nil
	}
	if exitCode >= 0 && ctx.Err() == nil {
		return exitCode, output, nil
	}
	return exitCode, output, runErr
}

func (m *Manager) git(ctx context.Context, repo Repo, args ...string) (int, string, error) {
	return m.gitAt(ctx, repo.Root(), args...)
}

func (m *Manager) gitAt(ctx context.Context, dir string, args ...string) (int, string, error) {
	return m.runner.Git(ctx, dir, args...)
}

func (m *Manager) warnf(format string, args ...any) {
	m.log(logging.Warn, logging.KindWarn, map[string]any{"msg": fmt.Sprintf(format, args...)})
}
