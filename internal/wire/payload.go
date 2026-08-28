package wire

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
