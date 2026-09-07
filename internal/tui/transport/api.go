// Package transport exposes the typed daemon boundary consumed by the TUI.
package transport

import (
	"context"
	"time"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// Snapshot is one coherent daemon-owned fleet snapshot. It excludes daemon
// connection and repository-worktree paths before crossing into the UI.
type Snapshot struct {
	ModelVersion     int
	Generation       uint64
	CapturedAt       time.Time
	Cursor           uint64
	Fresh            bool
	Stale            bool
	Runtime          Status
	Repositories     []Repository
	Seats            []Seat
	Sessions         []Session
	Beads            []Bead
	StatusCounts     []StatusCount
	RepositoryErrors []RepositoryError
}

// Repository is UI-visible repository identity without its local root path.
type Repository struct {
	Name   string
	Prefix string
	Branch string
}

// Seat is UI-visible seat state without its local worktree path.
type Seat struct {
	Name string
	Role string
	Repo string
	Task string
	Busy bool
}

// Status, Session, Bead, StatusCount, and RepositoryError are daemon snapshot
// values without connection or repository-path fields.
type (
	Status          = wire.StatusResult
	Session         = wire.SessionResult
	Bead            = wire.BeadResult
	StatusCount     = wire.StatusCount
	RepositoryError = wire.RepositoryError
)

// DaemonAPI supplies the read operations available to the TUI.
type DaemonAPI interface {
	Snapshot(context.Context) (Snapshot, error)
}

// ControlIntent reserves a typed vocabulary for future daemon controls.
// Snapshot transport neither interprets nor sends control intents.
type ControlIntent struct {
	Kind ControlKind
}

// ControlKind identifies a future daemon control without exposing wire commands.
type ControlKind string

const (
	ControlDispatch ControlKind = "dispatch"
	ControlReview   ControlKind = "review"
	ControlStart    ControlKind = "start"
	ControlStop     ControlKind = "stop"
)

// ErrorCode classifies a daemon-facing transport failure for the TUI.
type ErrorCode string

const (
	ErrorBadRequest     ErrorCode = ErrorCode(wire.CodeBadRequest)
	ErrorUnknownCommand ErrorCode = ErrorCode(wire.CodeUnknownCommand)
	ErrorNotFound       ErrorCode = ErrorCode(wire.CodeNotFound)
	ErrorConflict       ErrorCode = ErrorCode(wire.CodeConflict)
	ErrorUnavailable    ErrorCode = ErrorCode(wire.CodeUnavailable)
	ErrorSchemaMismatch ErrorCode = ErrorCode(wire.CodeSchemaMismatch)
	ErrorInternal       ErrorCode = ErrorCode(wire.CodeInternal)
)

// Error is a UI-safe daemon transport failure. It retains the daemon failure
// taxonomy but deliberately omits client messages, which can contain local
// socket paths.
type Error struct{ Code ErrorCode }

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return string(e.Code)
}
