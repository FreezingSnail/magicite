package wire

import "time"

// SessionResult describes one daemon-managed agent session.
type SessionResult struct {
	Handle        string `json:"handle"`
	Repo          string `json:"repo"`
	Task          string `json:"task"`
	Role          string `json:"role"`
	Seat          string `json:"seat"`
	Backend       string `json:"backend"`
	Model         string `json:"model"`
	Status        string `json:"status"`
	Phase         string `json:"phase"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// StatusResult describes daemon availability and managed sessions.
type StatusResult struct {
	Version        string          `json:"version"`
	Schema         int             `json:"schema"`
	Running        bool            `json:"running"`
	Draining       bool            `json:"draining"`
	Repos          int             `json:"repos"`
	ImplementerCap int             `json:"implementer_cap"`
	Sessions       []SessionResult `json:"sessions"`
}

// SeatResult describes one fleet seat.
type SeatResult struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Repo     string `json:"repo"`
	Worktree string `json:"worktree"`
	Task     string `json:"task"`
	Busy     bool   `json:"busy"`
}

// TaskResult describes one dispatchable task.
type TaskResult struct {
	ID         string   `json:"id"`
	Repo       string   `json:"repo"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Difficulty string   `json:"difficulty"`
	Priority   int      `json:"priority"`
	Labels     []string `json:"labels"`
}

// RepoResult describes one configured repository.
type RepoResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Prefix string `json:"prefix"`
	Branch string `json:"branch"`
}

// TasksParams filters listed tasks.
type TasksParams struct {
	Repo string `json:"repo"`
	All  bool   `json:"all"`
}

// DispatchParams identifies work to dispatch.
type DispatchParams struct {
	Task string `json:"task"`
	Repo string `json:"repo"`
	Role string `json:"role"`
}

// DispatchResult identifies a dispatched session.
type DispatchResult struct {
	Handle string `json:"handle"`
	Repo   string `json:"repo"`
	Task   string `json:"task"`
	Role   string `json:"role"`
	Seat   string `json:"seat"`
}

// StopParams controls graceful or hard shutdown.
type StopParams struct {
	Hard bool `json:"hard"`
}

// StopResult describes shutdown state.
type StopResult struct {
	Mode     string `json:"mode"`
	Sessions int    `json:"sessions"`
	Draining bool   `json:"draining"`
}

// ReviewParams identifies work to review.
type ReviewParams struct {
	Epic string `json:"epic"`
	Repo string `json:"repo"`
}

// ReviewResult describes a review session.
type ReviewResult struct {
	Epic   string `json:"epic"`
	Repo   string `json:"repo"`
	Handle string `json:"handle"`
	Held   bool   `json:"held"`
}

// SnapshotResult is one coherent daemon-owned fleet snapshot.
type SnapshotResult struct {
	ModelVersion     int               `json:"model_version"`
	Generation       uint64            `json:"generation"`
	CapturedAt       time.Time         `json:"captured_at"`
	Cursor           uint64            `json:"cursor"`
	Fresh            bool              `json:"fresh"`
	Stale            bool              `json:"stale"`
	Runtime          StatusResult      `json:"runtime"`
	Repositories     []RepoResult      `json:"repositories"`
	Seats            []SeatResult      `json:"seats"`
	Sessions         []SessionResult   `json:"sessions"`
	Beads            []BeadResult      `json:"beads"`
	StatusCounts     []StatusCount     `json:"status_counts"`
	RepositoryErrors []RepositoryError `json:"repository_errors"`
}

// BeadResult describes a complete bead and its daemon-derived workflow state.
type BeadResult struct {
	ID                 string             `json:"id"`
	Repo               string             `json:"repo"`
	Title              string             `json:"title"`
	Description        *string            `json:"description"`
	Design             *string            `json:"design"`
	AcceptanceCriteria *string            `json:"acceptance_criteria"`
	Status             string             `json:"status"`
	Priority           int                `json:"priority"`
	IssueType          string             `json:"issue_type"`
	Assignee           *string            `json:"assignee"`
	Owner              *string            `json:"owner"`
	Parent             *string            `json:"parent"`
	CreatedAt          *time.Time         `json:"created_at"`
	CreatedBy          *string            `json:"created_by"`
	UpdatedAt          *time.Time         `json:"updated_at"`
	StartedAt          *time.Time         `json:"started_at"`
	ClosedAt           *time.Time         `json:"closed_at"`
	DeferredUntil      *time.Time         `json:"deferred_until"`
	CloseReason        *string            `json:"close_reason"`
	DependencyCount    int                `json:"dependency_count"`
	DependentCount     int                `json:"dependent_count"`
	CommentCount       int                `json:"comment_count"`
	Dependencies       []DependencyResult `json:"dependencies"`
	Labels             []string           `json:"labels"`
	Staged             bool               `json:"staged"`
	Dispatch           Eligibility        `json:"dispatch"`
	Review             Eligibility        `json:"review"`
}

// DependencyResult summarizes one bead dependency edge.
type DependencyResult struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// StatusCount reports the number of beads in one status.
type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// RepositoryError reports a named repository snapshot failure.
type RepositoryError struct {
	Repository string `json:"repository"`
	Error      string `json:"error"`
}

// Eligibility states whether a workflow operation is currently available.
type Eligibility struct {
	Eligible bool    `json:"eligible"`
	Reason   *string `json:"reason"`
}
