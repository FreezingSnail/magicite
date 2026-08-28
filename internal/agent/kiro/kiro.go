// Package kiro adapts Kiro's chat command to the agent runtime.
package kiro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/connorfranc/magicite/internal/agent"
	executil "github.com/connorfranc/magicite/internal/exec"
	"github.com/connorfranc/magicite/internal/logging"
)

const defaultExecutable = "kiro-cli-chat"

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// Options configures a Kiro adapter.
type Options struct {
	Executable string
	AgentsDir  string
	Env        []string
}

// Adapter runs Kiro chat sessions and retains their local state.
type Adapter struct {
	executable string
	agentsDir  string
	env        []string
	store      *agent.Store[*run]
}

type run struct {
	workdir string
	session *executil.Session

	mu     sync.RWMutex
	output bytes.Buffer
	status agent.Status
	done   bool
}

// New creates a Kiro adapter.
func New(opts Options) *Adapter {
	if opts.Executable == "" {
		opts.Executable = defaultExecutable
	}
	return &Adapter{
		executable: opts.Executable,
		agentsDir:  opts.AgentsDir,
		env:        append([]string(nil), opts.Env...),
		store:      agent.NewStore[*run]("kiro"),
	}
}

// Name identifies this adapter's backend.
func (a *Adapter) Name() string { return "kiro" }

// Executable returns the configured Kiro command.
func (a *Adapter) Executable() string { return a.executable }

// Run starts Kiro in spec.Workdir and returns its local handle.
func (a *Adapter) Run(ctx context.Context, spec agent.RunSpec) (agent.Handle, error) {
	if !ValidAgent(a.agentsDir, spec.Agent) {
		return "", fmt.Errorf("invalid Kiro agent %q", spec.Agent)
	}
	if !ValidModel(spec.Model) {
		return "", fmt.Errorf("invalid Kiro model %q", spec.Model)
	}
	if strings.TrimSpace(spec.Plan) == "" {
		return "", errors.New("empty Kiro plan")
	}
	if _, err := exec.LookPath(a.executable); err != nil {
		return "", fmt.Errorf("%w: %s", agent.ErrExecutableMissing, a.executable)
	}
	workdir, err := filepath.Abs(spec.Workdir)
	if err != nil {
		return "", fmt.Errorf("absolute Kiro worktree: %w", err)
	}
	if !ValidEffort(spec.Effort) && spec.Effort != "" {
		logging.Event(logging.Warn, logging.KindWarn, map[string]any{
			"backend": "kiro", "effort": spec.Effort, "message": "invalid effort omitted",
		})
	}

	args := RunArgs(a.executable, spec.Model, spec.Agent, spec.Effort, spec.Plan)
	session, err := executil.Start(ctx, executil.Spec{
		Dir: workdir, Name: args[0], Args: args[1:], Env: a.env,
	})
	if err != nil {
		return "", err
	}
	state := &run{workdir: workdir, session: session, status: agent.StatusRunning}
	handle, _ := a.store.Add(state)
	go a.collect(handle, state, spec.Notify)
	return handle, nil
}

func (a *Adapter) collect(handle agent.Handle, state *run, notify agent.Notifier) {
	var copies sync.WaitGroup
	copies.Add(2)
	go a.copyOutput(&copies, state, state.session.Stdout())
	go a.copyOutput(&copies, state, state.session.Stderr())
	exitCode, waitErr := state.session.Wait()
	copies.Wait()

	state.mu.Lock()
	output := stripANSI(state.output.String())
	state.output.Reset()
	_, _ = state.output.WriteString(output)
	state.status = classify(exitCode, waitErr, output)
	state.done = true
	status := state.status
	state.mu.Unlock()
	if notify != nil {
		notify(handle, status)
	}
}

func (a *Adapter) copyOutput(copies *sync.WaitGroup, state *run, reader io.Reader) {
	defer copies.Done()
	var output bytes.Buffer
	_, _ = io.Copy(&output, reader)
	state.mu.Lock()
	_, _ = state.output.Write(output.Bytes())
	state.mu.Unlock()
}

func classify(exitCode int, waitErr error, output string) agent.Status {
	if agent.LimitedTail(output) {
		return agent.StatusLimited
	}
	if waitErr == nil && exitCode == 0 && strings.TrimSpace(output) != "" && !agent.FailureTail(output) {
		return agent.StatusCompleted
	}
	return agent.StatusFailed
}

// Complete reports a run's current or terminal status.
func (a *Adapter) Complete(_ context.Context, handle agent.Handle) (agent.Status, error) {
	state, ok := a.store.Get(handle)
	if !ok {
		return agent.StatusFailed, nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	if !state.done {
		select {
		case <-state.session.Done():
			return agent.StatusFailed, nil
		default:
		}
	}
	return state.status, nil
}

// Output returns the transcript collected for handle.
func (a *Adapter) Output(_ context.Context, handle agent.Handle) (string, error) {
	state, ok := a.store.Get(handle)
	if !ok {
		return "", fmt.Errorf("%w: %s", agent.ErrUnknownHandle, handle)
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return stripANSI(state.output.String()), nil
}

// Delete terminates a run's process group and forgets its local state.
func (a *Adapter) Delete(_ context.Context, handle agent.Handle) error {
	state, ok := a.store.Get(handle)
	if !ok {
		return nil
	}
	state.mu.Lock()
	state.done = true
	state.mu.Unlock()
	if err := state.session.Terminate(100 * time.Millisecond); err != nil {
		return err
	}
	a.store.Delete(handle)
	return nil
}

// UsageLimited reports whether handle ended from a usage limit.
func (a *Adapter) UsageLimited(_ context.Context, handle agent.Handle) bool {
	status, _ := a.Complete(context.Background(), handle)
	return status == agent.StatusLimited
}

func stripANSI(output string) string { return ansiEscape.ReplaceAllString(output, "") }
