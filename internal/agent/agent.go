// Package agent defines interfaces for agent backends and their runtime facade.
package agent

import (
	"context"
	"errors"
)

// Status describes an agent run's current or final state.
type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusLimited   Status = "limited"
)

// Handle identifies a run within an adapter.
type Handle string

// RunSpec configures a backend run.
type RunSpec struct {
	Workdir string
	Model   string
	Agent   string
	Effort  string
	Plan    string
	Notify  Notifier
}

// Notifier receives agent status updates.
type Notifier func(Handle, Status)

// FileDiff describes one changed file.
type FileDiff struct {
	File      string
	Patch     string
	Status    string
	Additions int
	Deletions int
}

// Adapter implements one agent backend.
type Adapter interface {
	Name() string
	Executable() string
	Run(context.Context, RunSpec) (Handle, error)
	Complete(context.Context, Handle) (Status, error)
	Diff(context.Context, Handle) ([]FileDiff, error)
	Output(context.Context, Handle) (string, error)
	Delete(context.Context, Handle) error
	UsageLimited(context.Context, Handle) bool
}

var (
	// ErrInvalidAdapter reports an adapter missing required registration data.
	ErrInvalidAdapter = errors.New("invalid agent adapter")
	// ErrDuplicateBackend reports a duplicate adapter name.
	ErrDuplicateBackend = errors.New("duplicate agent backend")
	// ErrUnknownBackend reports an unregistered adapter name.
	ErrUnknownBackend = errors.New("unknown agent backend")
	// ErrExecutableMissing reports an adapter executable absent from PATH.
	ErrExecutableMissing = errors.New("agent executable missing")
	// ErrUnknownHandle reports a handle not owned by this runtime.
	ErrUnknownHandle = errors.New("unknown agent handle")
)

// DefaultBackend resolves an empty backend name.
const DefaultBackend = "opencode"
