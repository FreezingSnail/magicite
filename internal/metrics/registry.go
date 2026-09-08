// Package metrics records daemon-local process-lifetime metrics.
package metrics

import (
	"sort"
	"sync"
	"time"
)

const (
	LifecyclePickup   = "pickup"
	LifecycleComplete = "complete"
	LifecycleLand     = "land"
	LifecycleClose    = "close"
	LifecycleReview   = "review"
	LifecycleVerdict  = "verdict"
	LifecycleRecovery = "recovery"
	LifecycleWarn     = "warn"
	LifecycleError    = "error"

	LandOK         = "ok"
	LandConflict   = "conflict"
	LandGateFailed = "gate_failed"
	LandFailed     = "failed"

	RoleConcierge   = "concierge"
	RoleDesigner    = "designer"
	RoleImplementer = "implementer"
	RoleReviewer    = "reviewer"
	RoleRepairer    = "repairer"
)

var lifecycleKeys = []string{
	LifecyclePickup,
	LifecycleComplete,
	LifecycleLand,
	LifecycleClose,
	LifecycleReview,
	LifecycleVerdict,
	LifecycleRecovery,
	LifecycleWarn,
	LifecycleError,
}

var landKeys = []string{LandOK, LandConflict, LandGateFailed, LandFailed}

var roleKeys = []string{RoleConcierge, RoleDesigner, RoleImplementer, RoleReviewer, RoleRepairer}

// SessionGauges contains current and terminal session counts.
type SessionGauges struct {
	Active    uint64
	Peak      uint64
	Completed uint64
	Failed    uint64
}

// RoleTotal contains all terminal sessions and their cumulative elapsed time
// for one fixed agent role.
type RoleTotal struct {
	Sessions uint64
	Duration time.Duration
}

// QueueDepth is one repository's ready-task depth in the latest queue sample.
type QueueDepth struct {
	Repository string
	Depth      int
}

// Snapshot is an immutable point-in-time copy of Registry state. Its maps and
// slices are owned by the snapshot and can be changed without affecting the
// registry or another snapshot.
type Snapshot struct {
	StartedAt      time.Time
	Uptime         time.Duration
	Lifecycle      map[string]uint64
	Land           map[string]uint64
	Sessions       SessionGauges
	Roles          map[string]RoleTotal
	Queue          []QueueDepth
	QueueTotal     int
	QueueSampledAt *time.Time
}

// Registry retains process-lifetime metrics. It has no external dependencies.
type Registry struct {
	mu sync.RWMutex

	now       func() time.Time
	startedAt time.Time
	lifecycle map[string]uint64
	land      map[string]uint64
	sessions  SessionGauges
	roles     map[string]RoleTotal
	queue     map[string]int
	sampledAt *time.Time
}

// NewRegistry constructs a Registry. When supplied, now provides the clock for
// its creation and snapshots; omitting it uses time.Now. A nil clock uses
// time.Now.
func NewRegistry(now ...func() time.Time) *Registry {
	clock := time.Now
	if len(now) > 0 && now[0] != nil {
		clock = now[0]
	}
	return &Registry{
		now:       clock,
		startedAt: clock(),
		lifecycle: counters(lifecycleKeys),
		land:      counters(landKeys),
		roles:     roleTotals(roleKeys),
		queue:     make(map[string]int),
	}
}

// RecordLifecycle increments key when it is one of the fixed lifecycle keys.
// It reports whether key was accepted.
func (r *Registry) RecordLifecycle(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.lifecycle[key]; !ok {
		return false
	}
	r.lifecycle[key]++
	return true
}

// RecordLand increments key when it is one of the fixed landing outcome keys.
// It reports whether key was accepted.
func (r *Registry) RecordLand(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.land[key]; !ok {
		return false
	}
	r.land[key]++
	return true
}

// StartSession records a session becoming active. Peak is process-lifetime and
// never decreases.
func (r *Registry) StartSession() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions.Active++
	if r.sessions.Active > r.sessions.Peak {
		r.sessions.Peak = r.sessions.Active
	}
}

// FinishSession records one terminal session. Unknown roles are rejected and
// leave every metric unchanged. Negative elapsed durations count as zero.
func (r *Registry) FinishSession(role string, elapsed time.Duration, failed bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	total, ok := r.roles[role]
	if !ok {
		return false
	}
	if elapsed < 0 {
		elapsed = 0
	}
	total.Sessions++
	total.Duration += elapsed
	r.roles[role] = total
	if r.sessions.Active > 0 {
		r.sessions.Active--
	}
	if failed {
		r.sessions.Failed++
	} else {
		r.sessions.Completed++
	}
	return true
}

// SetQueue replaces the latest queue sample. Repository names are retained
// only for this sample, so changing repositories cannot accumulate labels over
// the process lifetime. Negative depths clamp to zero.
func (r *Registry) SetQueue(depths map[string]int, sampledAt time.Time) {
	queue := make(map[string]int, len(depths))
	for repository, depth := range depths {
		if repository == "" {
			continue
		}
		if depth < 0 {
			depth = 0
		}
		queue[repository] = depth
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue = queue
	captured := sampledAt
	r.sampledAt = &captured
}

// Snapshot returns a deep copy of every mutable collection in the registry.
func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := Snapshot{
		StartedAt: r.startedAt,
		Uptime:    nonNegativeDuration(r.now().Sub(r.startedAt)),
		Lifecycle: cloneCounters(r.lifecycle),
		Land:      cloneCounters(r.land),
		Sessions:  r.sessions,
		Roles:     cloneRoleTotals(r.roles),
		Queue:     queueDepths(r.queue),
	}
	for _, depth := range snapshot.Queue {
		snapshot.QueueTotal += depth.Depth
	}
	if r.sampledAt != nil {
		captured := *r.sampledAt
		snapshot.QueueSampledAt = &captured
	}
	return snapshot
}

func counters(keys []string) map[string]uint64 {
	values := make(map[string]uint64, len(keys))
	for _, key := range keys {
		values[key] = 0
	}
	return values
}

func roleTotals(keys []string) map[string]RoleTotal {
	values := make(map[string]RoleTotal, len(keys))
	for _, key := range keys {
		values[key] = RoleTotal{}
	}
	return values
}

func cloneCounters(values map[string]uint64) map[string]uint64 {
	clone := make(map[string]uint64, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneRoleTotals(values map[string]RoleTotal) map[string]RoleTotal {
	clone := make(map[string]RoleTotal, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func queueDepths(queue map[string]int) []QueueDepth {
	depths := make([]QueueDepth, 0, len(queue))
	for repository, depth := range queue {
		depths = append(depths, QueueDepth{Repository: repository, Depth: depth})
	}
	sort.Slice(depths, func(left, right int) bool {
		return depths[left].Repository < depths[right].Repository
	})
	return depths
}

func nonNegativeDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}
