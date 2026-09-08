// Package daemon assembles magicite's production daemon.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/dispatch"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/metrics"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/server"
	"github.com/FreezingSnail/magicite/internal/wire"
)

// Deps supplies the daemon dependencies exposed through server.Core.
type Deps struct {
	Config       config.Config
	Log          logging.Logger
	Dispatcher   *dispatch.Dispatcher
	Beads        dispatch.Beads
	Repos        dispatch.Repos
	Gate         dispatch.Gate
	Bus          *server.Bus
	Metrics      *metrics.Registry
	QueueSampler *QueueSampler
	Version      string
}

// DepsError identifies an incomplete daemon Core dependency set.
type DepsError struct{ Field string }

func (e *DepsError) Error() string { return fmt.Sprintf("daemon: %s is required", e.Field) }

const (
	snapshotModelVersion = 1
	snapshotTTL          = time.Second
	snapshotFanout       = 4
)

type coreSnapshotRaw struct {
	Beads []bd.Bead
}

type core struct {
	config     config.Config
	log        logging.Logger
	dispatcher *dispatch.Dispatcher
	beads      dispatch.Beads
	repos      dispatch.Repos
	gate       dispatch.Gate
	bus        *server.Bus
	metrics    *metrics.Registry
	queue      *QueueSampler
	version    string
	snapshots  *snapshotCoordinator[coreSnapshotRaw]

	mu      sync.RWMutex
	running bool
}

// NewCore creates the server capability adapter for a Dispatcher.
func NewCore(d Deps) (server.Core, error) {
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"Dispatcher", d.Dispatcher},
		{"Beads", d.Beads},
		{"Repos", d.Repos},
		{"Gate", d.Gate},
		{"Bus", d.Bus},
		{"Metrics", d.Metrics},
		{"QueueSampler", d.QueueSampler},
	} {
		if nilValue(dependency.value) {
			return nil, &DepsError{Field: dependency.name}
		}
	}
	core := &core{config: d.Config, log: d.Log, dispatcher: d.Dispatcher, beads: d.Beads, repos: d.Repos, gate: d.Gate, bus: d.Bus, metrics: d.Metrics, queue: d.QueueSampler, version: d.Version}
	cache := newSnapshotCache(snapshotTTL, time.Now, cloneCoreSnapshotState)
	core.snapshots = newSnapshotCoordinator(cache, newSnapshotRefresher(snapshotFanout, core.snapshotRead))
	return core, nil
}

func nilValue(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func (c *core) Status(ctx context.Context) (wire.StatusResult, error) {
	repos := c.repos.List(ctx)
	sessions := c.dispatcher.Sessions()
	result := wire.StatusResult{Version: c.version, Schema: wire.Schema, Draining: c.dispatcher.Draining(), Repos: len(repos), ImplementerCap: c.dispatcher.RoleCap(dispatch.Implementer), Sessions: make([]wire.SessionResult, 0, len(sessions))}
	c.mu.RLock()
	result.Running = c.running
	c.mu.RUnlock()
	for _, session := range sessions {
		uptime := int64(time.Since(session.Started).Seconds())
		if uptime < 0 {
			uptime = 0
		}
		result.Sessions = append(result.Sessions, wire.SessionResult{Handle: session.Handle, Repo: session.Repo.Name, Task: session.Task, Role: string(session.Role), Seat: session.Seat, Backend: session.Backend, Model: session.Model, Status: string(session.Status), Phase: session.Phase, UptimeSeconds: uptime})
	}
	return result, nil
}

// Metrics samples ready queues and combines process, queue, and bus telemetry.
func (c *core) Metrics(ctx context.Context) (wire.MetricsResult, error) {
	sample := c.queue.Sample(ctx)
	depths := make(map[string]int, len(sample.Repositories))
	for _, repository := range sample.Repositories {
		depths[repository.Repo.Name] = repository.Ready
	}
	c.metrics.SetQueue(depths, sample.CapturedAt)
	return metricsView(c.metrics.Snapshot(), c.bus.Metrics()), nil
}

func metricsView(snapshot metrics.Snapshot, bus server.BusMetrics) wire.MetricsResult {
	result := wire.MetricsResult{
		StartedAt:     snapshot.StartedAt,
		UptimeSeconds: int64(snapshot.Uptime / time.Second),
		Lifecycle:     make([]wire.MetricsCount, 0, 9),
		Land:          make([]wire.MetricsCount, 0, 4),
		Sessions: wire.SessionGauges{
			Active: snapshot.Sessions.Active, Peak: snapshot.Sessions.Peak,
			Completed: snapshot.Sessions.Completed, Failed: snapshot.Sessions.Failed,
		},
		Roles:          make([]wire.RoleDuration, 0, 5),
		Queue:          make([]wire.RepoQueueDepth, 0, len(snapshot.Queue)),
		QueueTotal:     snapshot.QueueTotal,
		QueueSampledAt: snapshot.QueueSampledAt,
		Bus:            wire.BusMetrics{Published: bus.Published, Dropped: bus.Dropped},
	}
	for _, key := range []string{metrics.LifecyclePickup, metrics.LifecycleComplete, metrics.LifecycleLand, metrics.LifecycleClose, metrics.LifecycleReview, metrics.LifecycleVerdict, metrics.LifecycleRecovery, metrics.LifecycleWarn, metrics.LifecycleError} {
		result.Lifecycle = append(result.Lifecycle, wire.MetricsCount{Key: key, Count: snapshot.Lifecycle[key]})
	}
	for _, key := range []string{metrics.LandOK, metrics.LandConflict, metrics.LandGateFailed, metrics.LandFailed} {
		result.Land = append(result.Land, wire.MetricsCount{Key: key, Count: snapshot.Land[key]})
	}
	for _, role := range []string{metrics.RoleConcierge, metrics.RoleDesigner, metrics.RoleImplementer, metrics.RoleReviewer, metrics.RoleRepairer} {
		total := snapshot.Roles[role]
		result.Roles = append(result.Roles, wire.RoleDuration{Role: role, Sessions: total.Sessions, TotalSeconds: int64(total.Duration / time.Second)})
	}
	for _, queue := range snapshot.Queue {
		result.Queue = append(result.Queue, wire.RepoQueueDepth{Repo: queue.Repository, Depth: queue.Depth})
	}
	return result
}

// Snapshot returns the complete daemon-owned fleet model. Repository reads are
// coalesced and retained by the coordinator; composing the returned raw state
// performs no further repository operations.
func (c *core) Snapshot(ctx context.Context) (wire.SnapshotResult, error) {
	repositories := c.repos.List(ctx)
	state, err := c.snapshots.Snapshot(ctx, repositories)
	if err != nil {
		return wire.SnapshotResult{}, classify(err)
	}

	runtime, err := c.Status(ctx)
	if err != nil {
		return wire.SnapshotResult{}, err
	}
	seats, err := c.Seats(ctx)
	if err != nil {
		return wire.SnapshotResult{}, err
	}
	result := wire.SnapshotResult{
		ModelVersion:     snapshotModelVersion,
		Generation:       state.Generation,
		CapturedAt:       time.Now().UTC(),
		Cursor:           c.bus.Last(),
		Fresh:            state.Fresh,
		Stale:            state.Stale,
		Runtime:          runtime,
		Repositories:     snapshotRepositories(repositories),
		Seats:            seats,
		Sessions:         append([]wire.SessionResult{}, runtime.Sessions...),
		Beads:            make([]wire.BeadResult, 0),
		StatusCounts:     make([]wire.StatusCount, 0),
		RepositoryErrors: make([]wire.RepositoryError, 0, len(state.Errors)),
	}
	counts := make(map[string]int)
	for _, record := range state.Records {
		for _, bead := range record.Value.Beads {
			result.Beads = append(result.Beads, beadView(record.Repo, bead, workflowFacts{}))
			counts[bead.Status]++
		}
	}
	for status, count := range counts {
		result.StatusCounts = append(result.StatusCounts, wire.StatusCount{Status: status, Count: count})
	}
	for _, repositoryError := range state.Errors {
		result.RepositoryErrors = append(result.RepositoryErrors, wire.RepositoryError{Repository: repositoryError.Repo.Name, Error: repositoryError.Err.Error()})
	}
	return result, nil
}

func (c *core) snapshotRead(ctx context.Context, repository repo.Repo) (coreSnapshotRaw, error) {
	reader, ok := c.beads.(interface {
		Query(context.Context, repo.Repo, string) ([]bd.Bead, error)
	})
	if !ok {
		return coreSnapshotRaw{}, fmt.Errorf("%w: snapshot query is unavailable", server.ErrUnavailable)
	}
	beads, err := reader.Query(ctx, repository, "")
	if err != nil {
		return coreSnapshotRaw{}, err
	}
	return coreSnapshotRaw{Beads: cloneSnapshotBeads(beads)}, nil
}

func cloneCoreSnapshotState(state snapshotState[coreSnapshotRaw]) snapshotState[coreSnapshotRaw] {
	clone := snapshotState[coreSnapshotRaw]{
		Records:    make([]snapshotRecord[coreSnapshotRaw], len(state.Records)),
		Errors:     append([]snapshotRepositoryError{}, state.Errors...),
		Generation: state.Generation,
		Fresh:      state.Fresh,
		Stale:      state.Stale,
	}
	for index, record := range state.Records {
		clone.Records[index] = snapshotRecord[coreSnapshotRaw]{Repo: record.Repo, Value: coreSnapshotRaw{Beads: cloneSnapshotBeads(record.Value.Beads)}}
	}
	return clone
}

func cloneSnapshotBeads(beads []bd.Bead) []bd.Bead {
	clone := append([]bd.Bead{}, beads...)
	for index := range clone {
		clone[index].Labels = append([]string{}, clone[index].Labels...)
		clone[index].Comments = append([]string{}, clone[index].Comments...)
		clone[index].Dependencies = append([]bd.Dependency{}, clone[index].Dependencies...)
	}
	return clone
}

func snapshotRepositories(repositories []repo.Repo) []wire.RepoResult {
	result := make([]wire.RepoResult, 0, len(repositories))
	for _, repository := range repositories {
		result = append(result, wire.RepoResult{Name: repository.Name, Path: repository.Root, Prefix: repository.Prefix, Branch: repository.Branch})
	}
	return result
}

func (c *core) Seats(ctx context.Context) ([]wire.SeatResult, error) {
	sessions := c.dispatcher.Sessions()
	bySeat := make(map[string]dispatch.Session, len(sessions))
	for _, session := range sessions {
		bySeat[session.Seat] = session
	}
	result := make([]wire.SeatResult, 0)
	for _, configured := range []struct {
		role  string
		seats []config.SeatConfig
	}{
		{"implementer", c.config.Fleet.Seats},
		{"designer", c.config.Designer.Seats},
		{"repairer", c.config.Repairer.Seats},
		{"reviewer", c.config.Reviewer.Seats},
	} {
		for _, seat := range configured.seats {
			view := wire.SeatResult{Name: seat.Name, Role: configured.role}
			if session, busy := bySeat[seat.Name]; busy {
				view.Busy, view.Repo, view.Task = true, session.Repo.Name, session.Task
			}
			result = append(result, view)
		}
	}
	return result, nil
}

func (c *core) Tasks(ctx context.Context, p wire.TasksParams) ([]wire.TaskResult, error) {
	repositories, err := c.taskRepos(ctx, p.Repo)
	if err != nil {
		return nil, err
	}
	result := make([]wire.TaskResult, 0)
	if p.All {
		query, ok := c.beads.(interface {
			Query(context.Context, repo.Repo, string) ([]bd.Bead, error)
		})
		if !ok {
			return nil, fmt.Errorf("%w: task query is unavailable", server.ErrUnavailable)
		}
		for _, repository := range repositories {
			beads, err := query.Query(ctx, repository, "")
			if err != nil {
				return nil, classify(err)
			}
			for _, bead := range beads {
				if bead.Status != "closed" {
					result = append(result, taskResult(repository, bead))
				}
			}
		}
		return result, nil
	}
	for _, repository := range repositories {
		ready, err := c.beads.Ready(ctx, repository)
		if err != nil {
			return nil, classify(err)
		}
		for _, entry := range ready {
			priority, _ := strconv.Atoi(entry.Priority)
			result = append(result, wire.TaskResult{ID: entry.Task, Repo: repository.Name, Status: "open", Priority: priority, Labels: []string{}})
		}
	}
	return result, nil
}

func taskResult(repository repo.Repo, bead bd.Bead) wire.TaskResult {
	return wire.TaskResult{ID: bead.ID, Repo: repository.Name, Title: bead.Title, Status: bead.Status, Priority: bead.Priority, Labels: []string{}}
}

func (c *core) Repos(ctx context.Context) ([]wire.RepoResult, error) {
	repositories := c.repos.List(ctx)
	result := make([]wire.RepoResult, 0, len(repositories))
	for _, repository := range repositories {
		result = append(result, wire.RepoResult{Name: repository.Name, Path: repository.Root, Prefix: repository.Prefix, Branch: repository.Branch})
	}
	return result, nil
}

func (c *core) Dispatch(ctx context.Context, p wire.DispatchParams) (wire.DispatchResult, error) {
	repository, err := c.dispatchRepo(ctx, p.Repo, p.Task)
	if err != nil {
		return wire.DispatchResult{}, err
	}
	if _, err := c.beads.Show(ctx, repository, p.Task); err != nil {
		return wire.DispatchResult{}, classify(err)
	}
	role, handle, seat := dispatch.Implementer, "", ""
	switch p.Role {
	case "implement":
		role = dispatch.Implementer
		handle = c.dispatcher.Implement(ctx, repository, p.Task)
	case "design":
		role = dispatch.Designer
		handle = c.dispatcher.Design(ctx, repository, p.Task)
	case "repair":
		role = dispatch.Repairer
		seat = c.dispatcher.FreeSeat(role)
		handle = c.dispatcher.Repair(ctx, repository, seat, p.Task)
	case "review":
		role = dispatch.Reviewer
		handle = c.dispatcher.Review(ctx, repository, p.Task)
	default:
		return wire.DispatchResult{}, fmt.Errorf("%w: unsupported role %q", server.ErrBadRequest, p.Role)
	}
	if handle == "" {
		if c.dispatcher.ActiveCount(role) >= c.dispatcher.RoleCap(role) || c.dispatcher.FreeSeat(role) == "" {
			return wire.DispatchResult{}, fmt.Errorf("%w: %s capacity is full", server.ErrConflict, p.Role)
		}
		return wire.DispatchResult{}, fmt.Errorf("%w: dispatch %s", server.ErrUnavailable, p.Task)
	}
	if seat == "" {
		for _, session := range c.dispatcher.Sessions() {
			if session.Handle == handle {
				seat = session.Seat
				break
			}
		}
	}
	result := wire.DispatchResult{Handle: handle, Repo: repository.Name, Task: p.Task, Role: p.Role, Seat: seat}
	c.invalidateSnapshot()
	return result, nil
}

func (c *core) Start(ctx context.Context) (wire.StatusResult, error) {
	if _, err := c.dispatcher.Start(ctx); err != nil {
		return wire.StatusResult{}, classify(err)
	}
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
	c.invalidateSnapshot()
	return c.Status(ctx)
}

func (c *core) Stop(ctx context.Context, p wire.StopParams) (wire.StopResult, error) {
	c.dispatcher.Stop(ctx, p.Hard)
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()
	mode := "drain"
	if p.Hard {
		mode = "hard"
	}
	c.invalidateSnapshot()
	return wire.StopResult{Mode: mode, Sessions: len(c.dispatcher.Sessions()), Draining: c.dispatcher.Draining()}, nil
}

func (c *core) Review(ctx context.Context, p wire.ReviewParams) (wire.ReviewResult, error) {
	repository, err := c.dispatchRepo(ctx, p.Repo, p.Epic)
	if err != nil {
		return wire.ReviewResult{}, err
	}
	epic, err := c.gate.GateEpic(ctx, repository, p.Epic)
	if err != nil {
		return wire.ReviewResult{}, classify(err)
	}
	if epic == "" {
		return wire.ReviewResult{}, fmt.Errorf("%w: epic %s is not ready for review", server.ErrConflict, p.Epic)
	}
	handle := c.dispatcher.Review(ctx, repository, epic)
	if handle == "" {
		return wire.ReviewResult{}, fmt.Errorf("%w: review %s", server.ErrUnavailable, p.Epic)
	}
	result := wire.ReviewResult{Epic: epic, Repo: repository.Name, Handle: handle, Held: true}
	c.invalidateSnapshot()
	return result, nil
}

func (c *core) invalidateSnapshot() {
	if c.snapshots != nil {
		c.snapshots.cache.Invalidate()
	}
}

func (c *core) taskRepos(ctx context.Context, name string) ([]repo.Repo, error) {
	repositories := c.repos.List(ctx)
	if name == "" {
		return repositories, nil
	}
	repository, err := repo.GetIn(repositories, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", server.ErrNotFound, err)
	}
	return []repo.Repo{repository}, nil
}

func (c *core) dispatchRepo(ctx context.Context, name, task string) (repo.Repo, error) {
	if name != "" {
		repository, err := repo.GetIn(c.repos.List(ctx), name)
		if err != nil {
			return repo.Repo{}, fmt.Errorf("%w: %v", server.ErrNotFound, err)
		}
		return repository, nil
	}
	if repository, err := repo.ForBeadIn(c.repos.List(ctx), task); err == nil {
		return repository, nil
	}
	repository, err := c.repos.Current(ctx, "")
	if err != nil {
		return repo.Repo{}, fmt.Errorf("%w: %v", server.ErrNotFound, err)
	}
	return repository, nil
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", server.ErrUnavailable, err)
	}
	if bd.IsNotFound(err) || repo.IsNotFound(err) {
		return fmt.Errorf("%w: %v", server.ErrNotFound, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "unavailable") {
		return fmt.Errorf("%w: %v", server.ErrUnavailable, err)
	}
	return err
}
