package dispatch

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
)

// Log records a dispatcher lifecycle event.
type Log func(logging.Level, string, map[string]any)

// Deps contains every external dependency used by Dispatcher.
type Deps struct {
	Beads      Beads
	Workspaces Workspaces
	Lander     Lander
	Runner     Runner
	Repos      Repos
	Gate       Gate
	Clock      Clock
	Config     config.Config
	Logger     Log
}

// MissingDependencyError reports incomplete dispatcher wiring.
type MissingDependencyError struct {
	Dependency string
}

func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("dispatch: missing dependency %s", e.Dependency)
}

// Dispatcher holds immutable external dependencies and later-owned state.
type Dispatcher struct {
	beads         Beads
	workspaces    Workspaces
	lander        Lander
	runner        Runner
	repos         Repos
	gate          Gate
	clock         Clock
	config        config.Config
	log           Log
	sessionsMu    sync.RWMutex
	sessions      map[string]Session
	stateMu       sync.Mutex
	tickInFlight  bool
	draining      bool
	pendingNotify bool
	repoWarnedAt  map[string]time.Time

	lifecycleMu        sync.Mutex
	callbacksInstalled bool
	running            bool
	ticker             Ticker
	tickCancel         context.CancelFunc
	drainDone          chan struct{}
	drainClosed        bool
}

// New constructs a Dispatcher after validating every port.
func New(deps Deps) (*Dispatcher, error) {
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"Beads", deps.Beads},
		{"Workspaces", deps.Workspaces},
		{"Lander", deps.Lander},
		{"Runner", deps.Runner},
		{"Repos", deps.Repos},
		{"Gate", deps.Gate},
		{"Clock", deps.Clock},
	} {
		if nilDependency(dependency.value) {
			return nil, &MissingDependencyError{Dependency: dependency.name}
		}
	}
	if deps.Logger == nil {
		deps.Logger = logging.Event
	}
	logger := deps.Logger
	var logMu sync.Mutex
	log := func(level logging.Level, kind string, fields map[string]any) {
		logMu.Lock()
		defer logMu.Unlock()
		logger(level, kind, fields)
	}
	return &Dispatcher{
		beads:        deps.Beads,
		workspaces:   deps.Workspaces,
		lander:       deps.Lander,
		runner:       deps.Runner,
		repos:        deps.Repos,
		gate:         deps.Gate,
		clock:        deps.Clock,
		config:       deps.Config,
		log:          log,
		sessions:     make(map[string]Session),
		repoWarnedAt: make(map[string]time.Time),
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
