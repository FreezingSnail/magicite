// Package dispatch coordinates repository work through narrow, testable ports.
package dispatch

import (
	"context"
	"time"

	"github.com/FreezingSnail/magicite/internal/repo"
)

// Role identifies a dispatch role.
type Role string

const (
	Implementer Role = "implementer"
	Designer    Role = "designer"
	Repairer    Role = "repairer"
	Reviewer    Role = "reviewer"
)

// Outcome is an agent session's terminal result.
type Outcome = SessionStatus

const (
	Completed Outcome = "completed"
	Limited   Outcome = "limited"
)

// LandResult is a landing attempt's stable outcome.
type LandResult string

const (
	LandOK         LandResult = "ok"
	LandConflict   LandResult = "conflict"
	LandGateFailed LandResult = "gate-failed"
	LandFailed     LandResult = "failed"
)

// SyncResult is a workspace sync result relevant to dispatch.
type SyncResult string

const (
	SyncOK       SyncResult = "ok"
	SyncConflict SyncResult = "conflict"
)

// ReadyEntry identifies one ready task and its source repository.
type ReadyEntry struct {
	Repo     repo.Repo
	Task     string
	Priority string
}

// RepoReady is one repository's ready queue.
type RepoReady struct {
	Repo    repo.Repo
	Entries []ReadyEntry
}

// Spec is the task content needed to build an agent plan.
type Spec struct {
	Title, Description, Design, Acceptance string
}

// RunSpec configures one agent session without exposing an agent runtime type.
type RunSpec struct {
	Workdir, Backend, Model, Agent, Effort, Plan string
}

// Diff is one changed file returned by a completed agent session.
type Diff struct {
	File, Patch, Status  string
	Additions, Deletions int
}

// Stamp records task provenance for a landed change.
type Stamp struct {
	Model, Backend, Difficulty, Effort, Agent string
	Repo, Seat, Task, Harness, HarnessRev     string
}

// Beads supplies repository-scoped task operations.
type Beads interface {
	Ready(context.Context, repo.Repo) ([]ReadyEntry, error)
	Show(context.Context, repo.Repo, string) (Spec, error)
	Claim(context.Context, repo.Repo, string) error
	Release(context.Context, repo.Repo, string) error
	Close(context.Context, repo.Repo, string, string) error
	Comment(context.Context, repo.Repo, string, string) error
	Difficulty(context.Context, repo.Repo, string) (string, error)
	HumanOnly(context.Context, repo.Repo, string) (bool, error)
	InProgress(context.Context, repo.Repo) ([]string, error)
	OpenEpics(context.Context, repo.Repo) ([]string, error)
	EpicChildren(context.Context, repo.Repo, string) ([]string, error)
	EpicOpenChildren(context.Context, repo.Repo, string) ([]string, error)
	DriftFixTasks(context.Context, repo.Repo) ([]string, error)
	CancelAll(context.Context) error
}

// Workspaces manages per-seat worktrees.
type Workspaces interface {
	Ensure(context.Context, repo.Repo, string) (string, error)
	Path(repo.Repo, string) (string, error)
	Sync(context.Context, repo.Repo, string) (SyncResult, error)
}

// Lander integrates a completed seat branch.
type Lander interface {
	Land(context.Context, repo.Repo, string, Stamp) (LandResult, error)
	Landed(context.Context, repo.Repo, string) (bool, error)
	TaskLanded(context.Context, repo.Repo, string) (bool, error)
}

// Runner controls agent sessions.
type Runner interface {
	Run(context.Context, RunSpec) (string, error)
	Diff(context.Context, string) ([]Diff, error)
	Output(context.Context, string) (string, error)
	Delete(context.Context, string) error
	UsageLimited(context.Context, string) (bool, error)
}

// Repos supplies repositories known to the daemon.
type Repos interface {
	List(context.Context) []repo.Repo
	Current(context.Context, string) (repo.Repo, error)
}

// Gate supplies optional review and decomposition decisions.
type Gate interface {
	Hold(context.Context, repo.Repo) (bool, error)
	DueEpic(context.Context, repo.Repo, string) (string, error)
	GateEpic(context.Context, repo.Repo, string) (string, error)
	ReviewPlan(context.Context, repo.Repo, string) (RunSpec, error)
	NoteSession(string, repo.Repo, string)
	CompleteReview(context.Context, string, string) error
	AbortReview(context.Context, string, string) error
	DecompositionVerdict(context.Context, repo.Repo, string) error
}

// Ticker supplies timer events and can be stopped.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

// Clock supplies time without binding dispatch to real timers.
type Clock interface {
	Now() time.Time
	Ticker(time.Duration) Ticker
}
