// Package gate coordinates repository-scoped quality-gate state.
package gate

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
)

const defaultMaxRetries = 3

// Config configures optional quality gates.
type Config struct {
	Enabled      bool
	Model, Agent string
	MaxRetries   int
}

// Beads supplies the bead operations required by quality gates.
type Beads interface {
	Show(context.Context, repo.Repo, string) (bd.Bead, error)
	Labels(context.Context, repo.Repo, string) ([]string, error)
	EpicChildren(context.Context, repo.Repo, string) ([]bd.Bead, error)
	Query(context.Context, repo.Repo, string) ([]bd.Bead, error)
	Create(context.Context, repo.Repo, bd.CreateRequest) (string, error)
	Comment(context.Context, repo.Repo, string, string) error
	Close(context.Context, repo.Repo, string, string) error
}

// Git supplies repository-scoped git output.
type Git interface {
	Output(context.Context, repo.Repo, ...string) (int, string, error)
}

// Repos resolves repositories retained by active review sessions.
type Repos interface {
	Get(string) (repo.Repo, bool)
}

// Deps contains every external dependency used by Gate.
type Deps struct {
	Beads  Beads
	Git    Git
	Repos  Repos
	Config Config
	Log    logging.Logger
}

// MissingDependencyError reports incomplete gate wiring.
type MissingDependencyError struct {
	Dependency string
}

func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("gate: missing dependency %s", e.Dependency)
}

// InvalidConfigError reports a required enabled-gate configuration value.
type InvalidConfigError struct {
	Field string
}

func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("gate: config %s is empty", e.Field)
}

// Gate holds immutable ports and repository-scoped process-lifetime state.
type Gate struct {
	beads  Beads
	git    Git
	repos  Repos
	config Config
	log    logging.Logger

	mu        sync.Mutex
	attempted map[key]int
	exhausted map[key]bool
	tracked   map[string]key
	started   map[key]string
}

// New constructs a Gate after validating every required port.
func New(d Deps) (*Gate, error) {
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"Beads", d.Beads},
		{"Git", d.Git},
		{"Repos", d.Repos},
	} {
		if nilDependency(dependency.value) {
			return nil, &MissingDependencyError{Dependency: dependency.name}
		}
	}
	if d.Config.Enabled && d.Config.Model == "" {
		return nil, &InvalidConfigError{Field: "Model"}
	}
	if d.Config.Enabled && d.Config.Agent == "" {
		return nil, &InvalidConfigError{Field: "Agent"}
	}
	if d.Config.MaxRetries <= 0 {
		d.Config.MaxRetries = defaultMaxRetries
	}
	return &Gate{
		beads:     d.Beads,
		git:       d.Git,
		repos:     d.Repos,
		config:    d.Config,
		log:       d.Log,
		attempted: make(map[key]int),
		exhausted: make(map[key]bool),
		tracked:   make(map[string]key),
		started:   make(map[key]string),
	}, nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// Enabled reports whether quality gates are active.
func (g *Gate) Enabled() bool { return g.config.Enabled }

// MaxRetries reports the normalized retry limit.
func (g *Gate) MaxRetries() int { return g.config.MaxRetries }

// Reset clears all state for a zero repository, or only state belonging to r.
func (g *Gate) Reset(r repo.Repo) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if r == (repo.Repo{}) {
		clear(g.attempted)
		clear(g.exhausted)
		clear(g.tracked)
		clear(g.started)
		return
	}
	for k := range g.attempted {
		if k.repo == r.Name {
			delete(g.attempted, k)
		}
	}
	for k := range g.exhausted {
		if k.repo == r.Name {
			delete(g.exhausted, k)
		}
	}
	for handle, k := range g.tracked {
		if k.repo == r.Name {
			delete(g.tracked, handle)
		}
	}
	for k := range g.started {
		if k.repo == r.Name {
			delete(g.started, k)
		}
	}
}
