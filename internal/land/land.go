// Package land provides the shared ports and git seam for landing work.
package land

import (
	"context"
	"errors"
)

var (
	ErrInvalidOptions  = errors.New("invalid land options")
	ErrUnresolvedRepo  = errors.New("unresolved repository")
	ErrInvalidSeat     = errors.New("invalid seat")
	ErrMissingWorktree = errors.New("missing worktree")
	ErrBranchMissing   = errors.New("branch missing")
	ErrConflict        = errors.New("landing conflict")
	ErrGateFailed      = errors.New("landing gate failed")
	ErrNotLinear       = errors.New("non-linear history")
	ErrTaskUnstamped   = errors.New("task unstamped")
)

// Repo supplies repository metadata required by landing operations.
type Repo interface {
	Name() string
	Root() string
	Integration() string
}

// Workspace resolves a seat's branch and worktree for a repository.
type Workspace interface {
	Branch(repo Repo, seat string) (string, error)
	Path(repo Repo, seat string) (string, error)
}

// Runner runs git with an explicit working directory and argv.
type Runner interface {
	Git(ctx context.Context, dir string, args ...string) (int, string, error)
}

// Options configures a Pipeline.
type Options struct {
	Workspace Workspace
	Runner    Runner
	Gate      []string
	GateFunc  func(ctx context.Context, c *Context) (int, error)
	Log       func(level, msg string)
}

// Pipeline holds immutable shared dependencies for landing operations.
type Pipeline struct {
	workspace Workspace
	runner    Runner
	gate      []string
	gateFunc  func(ctx context.Context, c *Context) (int, error)
	log       func(level, msg string)
}

// New builds a Pipeline with the default verification gate.
func New(opts Options) (*Pipeline, error) {
	if opts.Workspace == nil || opts.Runner == nil {
		return nil, ErrInvalidOptions
	}

	gate := opts.Gate
	if len(gate) == 0 {
		gate = []string{"make", "check"}
	}
	log := opts.Log
	if log == nil {
		log = func(string, string) {}
	}

	return &Pipeline{
		workspace: opts.Workspace,
		runner:    opts.Runner,
		gate:      append([]string(nil), gate...),
		gateFunc:  opts.GateFunc,
		log:       log,
	}, nil
}
