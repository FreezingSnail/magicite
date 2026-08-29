// Package opencode adapts OpenCode's headless run command to the agent runtime.
package opencode

import (
	"context"
	"fmt"
	"io"
	osexec "os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/FreezingSnail/magicite/internal/agent"
	executil "github.com/FreezingSnail/magicite/internal/exec"
)

const defaultExecutable = "opencode"

// Options configures an OpenCode adapter.
type Options struct {
	Executable string
}

// Adapter runs OpenCode sessions and retains their local state.
type Adapter struct {
	executable string
	store      *agent.Store[*run]
}

type run struct {
	workdir string
	session *executil.Session
	scanner *Scanner

	mu        sync.RWMutex
	status    agent.Status
	limited   bool
	sessionID string
	done      bool
	finished  chan struct{}
}

// New creates an OpenCode adapter.
func New(opts Options) *Adapter {
	if opts.Executable == "" {
		opts.Executable = defaultExecutable
	}
	return &Adapter{
		executable: opts.Executable,
		store:      agent.NewStore[*run]("opencode"),
	}
}

// Name identifies this adapter's backend.
func (a *Adapter) Name() string { return "opencode" }

// Executable returns the configured OpenCode command.
func (a *Adapter) Executable() string { return a.executable }

// ValidEffort reports whether effort is safe to pass as an OpenCode variant.
func ValidEffort(effort string) bool {
	if effort == "" || strings.Contains(effort, "/") {
		return false
	}
	for _, r := range effort {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// RunArgs returns one argv slot per OpenCode run command argument.
func RunArgs(exe, workdir, model, agentName, effort, handle, plan string) []string {
	args := []string{exe, "run", "--dir", workdir, "-m", model}
	if ValidEffort(effort) {
		args = append(args, "--variant", effort)
	}
	if agentName != "" {
		args = append(args, "--agent", agentName)
	}
	args = append(args, "--format", "json", "--auto", "--title", handle, plan)
	return args
}

// Run starts one OpenCode session in spec.Workdir.
func (a *Adapter) Run(ctx context.Context, spec agent.RunSpec) (agent.Handle, error) {
	state := &run{
		workdir:  spec.Workdir,
		scanner:  NewScanner(),
		status:   agent.StatusRunning,
		finished: make(chan struct{}),
	}
	handle, _ := a.store.Add(state)

	if _, err := osexec.LookPath(a.executable); err != nil {
		a.store.Delete(handle)
		return "", fmt.Errorf("%w: %s", agent.ErrExecutableMissing, a.executable)
	}
	args := RunArgs(a.executable, spec.Workdir, spec.Model, spec.Agent, spec.Effort, string(handle), spec.Plan)
	session, err := executil.Start(ctx, executil.Spec{
		Dir: spec.Workdir, Name: args[0], Args: args[1:], Env: nil,
	})
	if err != nil {
		a.store.Delete(handle)
		return "", err
	}
	state.session = session
	go a.collect(handle, state, spec.Notify)
	return handle, nil
}

func (a *Adapter) collect(handle agent.Handle, state *run, notify agent.Notifier) {
	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		_, _ = io.Copy(state.scanner, state.session.Stdout())
	}()
	go func() {
		defer copies.Done()
		_, _ = io.Copy(io.Discard, state.session.Stderr())
	}()
	_, _ = state.session.Wait()
	copies.Wait()
	state.scanner.Flush()

	status := state.scanner.Status()
	limited := state.scanner.Limited()
	if limited {
		status = agent.StatusLimited
	} else if status == agent.StatusRunning {
		status = agent.StatusFailed
	}
	sessionID := state.scanner.SessionID()

	state.mu.Lock()
	state.status = status
	state.limited = limited
	state.sessionID = sessionID
	state.done = true
	state.mu.Unlock()
	if sessionID != "" {
		a.store.Alias(sessionID, handle)
	}
	close(state.finished)
	if notify != nil {
		notify(handle, status)
	}
}

// Complete reports a run's current or terminal status.
func (a *Adapter) Complete(ctx context.Context, handle agent.Handle) (agent.Status, error) {
	state, ok := a.state(handle)
	if !ok {
		return agent.StatusFailed, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-state.finished:
		state.mu.RLock()
		defer state.mu.RUnlock()
		return state.status, nil
	default:
	}
	select {
	case <-state.session.Done():
		select {
		case <-state.finished:
			state.mu.RLock()
			defer state.mu.RUnlock()
			return state.status, nil
		case <-ctx.Done():
			return agent.StatusFailed, ctx.Err()
		}
	default:
		return agent.StatusRunning, nil
	}
}

// Output returns the raw streamed transcript for handle.
func (a *Adapter) Output(_ context.Context, handle agent.Handle) (string, error) {
	state, ok := a.state(handle)
	if !ok {
		return "", fmt.Errorf("%w: %s", agent.ErrUnknownHandle, handle)
	}
	return state.scanner.Transcript(), nil
}

// Delete terminates a session process group, forgets its local state, and
// removes its OpenCode session when one was observed.
func (a *Adapter) Delete(ctx context.Context, handle agent.Handle) error {
	resolved, ok := a.store.Resolve(string(handle))
	if !ok {
		return nil
	}
	state, ok := a.store.Get(resolved)
	if !ok {
		return nil
	}
	if err := state.session.Terminate(100 * time.Millisecond); err != nil {
		return err
	}
	<-state.finished
	state.mu.RLock()
	sessionID := state.sessionID
	workdir := state.workdir
	state.mu.RUnlock()
	a.store.Delete(resolved)
	if sessionID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, stderr, _, err := executil.Run(ctx, workdir, a.executable, "session", "delete", sessionID)
	if err != nil {
		return fmt.Errorf("opencode session delete %q: %w: %s", sessionID, err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// UsageLimited reports whether handle ended from a usage limit.
func (a *Adapter) UsageLimited(_ context.Context, handle agent.Handle) bool {
	state, ok := a.state(handle)
	if !ok {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.limited
}

func (a *Adapter) state(handle agent.Handle) (*run, bool) {
	resolved, ok := a.store.Resolve(string(handle))
	if !ok {
		return nil, false
	}
	return a.store.Get(resolved)
}

var _ agent.Adapter = (*Adapter)(nil)
