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

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/dispatch"
	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
	"github.com/connorfranc/magicite/internal/server"
	"github.com/connorfranc/magicite/internal/wire"
)

// Deps supplies the daemon dependencies exposed through server.Core.
type Deps struct {
	Config     config.Config
	Log        logging.Logger
	Dispatcher *dispatch.Dispatcher
	Beads      dispatch.Beads
	Repos      dispatch.Repos
	Gate       dispatch.Gate
	Bus        *server.Bus
	Version    string
}

// DepsError identifies an incomplete daemon Core dependency set.
type DepsError struct{ Field string }

func (e *DepsError) Error() string { return fmt.Sprintf("daemon: %s is required", e.Field) }

type core struct {
	config     config.Config
	log        logging.Logger
	dispatcher *dispatch.Dispatcher
	beads      dispatch.Beads
	repos      dispatch.Repos
	gate       dispatch.Gate
	bus        *server.Bus
	version    string

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
	} {
		if nilValue(dependency.value) {
			return nil, &DepsError{Field: dependency.name}
		}
	}
	return &core{config: d.Config, log: d.Log, dispatcher: d.Dispatcher, beads: d.Beads, repos: d.Repos, gate: d.Gate, bus: d.Bus, version: d.Version}, nil
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
	return wire.DispatchResult{Handle: handle, Repo: repository.Name, Task: p.Task, Role: p.Role, Seat: seat}, nil
}

func (c *core) Start(ctx context.Context) (wire.StatusResult, error) {
	if _, err := c.dispatcher.Start(ctx); err != nil {
		return wire.StatusResult{}, classify(err)
	}
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
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
	return wire.ReviewResult{Epic: epic, Repo: repository.Name, Handle: handle, Held: true}, nil
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
