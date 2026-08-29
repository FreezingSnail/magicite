// Package conformance supplies adapter conformance fixtures and assertions.
package conformance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/agent"
)

const (
	ScenarioOK      = "scenario-ok"
	ScenarioDenied  = "scenario-denied"
	ScenarioLimited = "scenario-limited"
	ScenarioChild   = "scenario-child"
	ScenarioHang    = "scenario-hang"
)

const (
	pollInterval = 10 * time.Millisecond
	pollTimeout  = 2 * time.Second
)

// Case configures one adapter's shared conformance run.
type Case struct {
	Name    string
	Mode    string
	New     func(t *testing.T, executable string) agent.Adapter
	Workdir func(t *testing.T) string
}

// Run exercises adapter behavior against FakeCLI, never an installed agent.
func Run(t *testing.T, c Case) {
	t.Helper()
	if c.Name == "" || c.New == nil || c.Workdir == nil {
		t.Fatalf("conformance %q setup: Name, New, and Workdir are required", c.Name)
	}
	if c.Mode != "opencode" && c.Mode != "kiro" {
		t.Fatalf("conformance %s setup: unsupported mode %q", c.Name, c.Mode)
	}
	executable := FakeCLI(t)

	newAdapter := func(t *testing.T) agent.Adapter {
		t.Helper()
		adapter := c.New(t, executable)
		if adapter == nil {
			t.Fatalf("conformance %s setup: New returned nil", c.Name)
		}
		return adapter
	}
	run := func(t *testing.T, adapter agent.Adapter, scenario string, ctx context.Context) agent.Handle {
		t.Helper()
		workdir := c.Workdir(t)
		if workdir == "" {
			t.Fatalf("conformance %s %s: empty workdir", c.Name, scenario)
		}
		handle, err := adapter.Run(ctx, agent.RunSpec{
			Workdir: workdir,
			Model:   "fake-model",
			Effort:  "medium",
			Plan:    scenario,
		})
		if err != nil {
			t.Fatalf("conformance %s %s: Run: %v", c.Name, scenario, err)
		}
		return handle
	}

	t.Run(ScenarioOK, func(t *testing.T) {
		adapter := newAdapter(t)
		var notifications atomic.Int32
		workdir := c.Workdir(t)
		handle, err := adapter.Run(context.Background(), agent.RunSpec{
			Workdir: workdir,
			Model:   "fake-model",
			Effort:  "medium",
			Plan:    ScenarioOK,
			Notify:  func(agent.Handle, agent.Status) { notifications.Add(1) },
		})
		if err != nil {
			t.Fatalf("conformance %s %s: Run: %v", c.Name, ScenarioOK, err)
		}
		awaitStatus(t, c.Name, ScenarioOK, adapter, handle, agent.StatusCompleted)
		awaitOneNotification(t, c.Name, ScenarioOK, &notifications)
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: Delete: %v", c.Name, ScenarioOK, err)
		}
	})

	t.Run(ScenarioDenied, func(t *testing.T) {
		adapter := newAdapter(t)
		handle := run(t, adapter, ScenarioDenied, context.Background())
		awaitStatus(t, c.Name, ScenarioDenied, adapter, handle, agent.StatusFailed)
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: Delete: %v", c.Name, ScenarioDenied, err)
		}
	})

	t.Run(ScenarioLimited, func(t *testing.T) {
		adapter := newAdapter(t)
		handle := run(t, adapter, ScenarioLimited, context.Background())
		awaitStatus(t, c.Name, ScenarioLimited, adapter, handle, agent.StatusLimited)
		if !adapter.UsageLimited(context.Background(), handle) {
			t.Fatalf("conformance %s %s: UsageLimited = false", c.Name, ScenarioLimited)
		}
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: Delete: %v", c.Name, ScenarioLimited, err)
		}
	})

	t.Run("diff-output", func(t *testing.T) {
		adapter := newAdapter(t)
		handle := run(t, adapter, ScenarioOK, context.Background())
		awaitStatus(t, c.Name, ScenarioOK, adapter, handle, agent.StatusCompleted)
		diff, err := adapter.Diff(context.Background(), handle)
		if err != nil {
			t.Fatalf("conformance %s %s: Diff: %v", c.Name, ScenarioOK, err)
		}
		if !hasFiles(diff, "tracked.txt", "untracked.txt") {
			t.Fatalf("conformance %s %s: Diff files = %v, want tracked.txt and untracked.txt", c.Name, ScenarioOK, diff)
		}
		output, err := adapter.Output(context.Background(), handle)
		if err != nil {
			t.Fatalf("conformance %s %s: Output: %v", c.Name, ScenarioOK, err)
		}
		if strings.TrimSpace(output) == "" {
			t.Fatalf("conformance %s %s: Output empty", c.Name, ScenarioOK)
		}
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: Delete: %v", c.Name, ScenarioOK, err)
		}
	})

	t.Run("delete-child", func(t *testing.T) {
		adapter := newAdapter(t)
		workdir := c.Workdir(t)
		handle, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: workdir, Model: "fake-model", Effort: "medium", Plan: ScenarioChild})
		if err != nil {
			t.Fatalf("conformance %s %s: Run: %v", c.Name, ScenarioChild, err)
		}
		childPID := awaitPID(t, c.Name, ScenarioChild, pathJoin(workdir, ".fakeagent-child.pid"))
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: Delete: %v", c.Name, ScenarioChild, err)
		}
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: repeated Delete: %v", c.Name, ScenarioChild, err)
		}
		awaitPIDGone(t, c.Name, ScenarioChild, childPID)
	})

	t.Run("cancel", func(t *testing.T) {
		adapter := newAdapter(t)
		workdir := c.Workdir(t)
		ctx, cancel := context.WithCancel(context.Background())
		handle, err := adapter.Run(ctx, agent.RunSpec{Workdir: workdir, Model: "fake-model", Effort: "medium", Plan: ScenarioHang})
		if err != nil {
			cancel()
			t.Fatalf("conformance %s %s: Run: %v", c.Name, ScenarioHang, err)
		}
		pid := awaitPID(t, c.Name, ScenarioHang, pathJoin(workdir, ".fakeagent.pid"))
		cancel()
		awaitStatus(t, c.Name, ScenarioHang, adapter, handle, agent.StatusFailed)
		awaitPIDGone(t, c.Name, ScenarioHang, pid)
		if err := adapter.Delete(context.Background(), handle); err != nil {
			t.Fatalf("conformance %s %s: Delete after cancel: %v", c.Name, ScenarioHang, err)
		}
	})

	t.Run("unknown-handle", func(t *testing.T) {
		adapter := newAdapter(t)
		if _, err := adapter.Diff(context.Background(), "missing"); err == nil {
			t.Fatalf("conformance %s unknown-handle: Diff succeeded", c.Name)
		}
	})
}

func awaitStatus(t *testing.T, name, scenario string, adapter agent.Adapter, handle agent.Handle, want agent.Status) {
	t.Helper()
	deadline := time.NewTimer(pollTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	var got agent.Status
	var err error
	for {
		got, err = adapter.Complete(context.Background(), handle)
		if err == nil && got == want {
			return
		}
		if err == nil && got != agent.StatusRunning {
			t.Fatalf("conformance %s %s: Complete = %q, want %q", name, scenario, got, want)
		}
		select {
		case <-deadline.C:
			t.Fatalf("conformance %s %s: Complete = (%q, %v), want %q", name, scenario, got, err, want)
		case <-ticker.C:
		}
	}
}

func awaitOneNotification(t *testing.T, name, scenario string, count *atomic.Int32) {
	t.Helper()
	deadline := time.NewTimer(100 * time.Millisecond)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if got := count.Load(); got > 1 {
			t.Fatalf("conformance %s %s: notifications = %d, want 1", name, scenario, got)
		}
		select {
		case <-deadline.C:
			if got := count.Load(); got != 1 {
				t.Fatalf("conformance %s %s: notifications = %d, want 1", name, scenario, got)
			}
			return
		case <-ticker.C:
		}
	}
}

func hasFiles(diff []agent.FileDiff, names ...string) bool {
	seen := make(map[string]bool, len(diff))
	for _, file := range diff {
		seen[file.File] = true
	}
	for _, name := range names {
		if !seen[name] {
			return false
		}
	}
	return true
}

func pathJoin(dir, name string) string { return dir + string(os.PathSeparator) + name }

func awaitPID(t *testing.T, name, scenario, path string) int {
	t.Helper()
	deadline := time.NewTimer(pollTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr != nil || pid <= 0 {
				t.Fatalf("conformance %s %s: invalid pid file %s: %q", name, scenario, path, data)
			}
			return pid
		}
		if !os.IsNotExist(err) {
			t.Fatalf("conformance %s %s: read pid file: %v", name, scenario, err)
		}
		select {
		case <-deadline.C:
			t.Fatalf("conformance %s %s: pid file not written", name, scenario)
		case <-ticker.C:
		}
	}
}

func awaitPIDGone(t *testing.T, name, scenario string, pid int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		alive, err := processAlive(ctx, pid)
		if err != nil {
			if ctx.Err() != nil {
				alive = true
			} else {
				t.Fatalf("conformance %s %s: inspect pid %d: %v", name, scenario, pid, err)
			}
		}
		if !alive {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("conformance %s %s: descendant pid %d survived", name, scenario, pid)
		case <-ticker.C:
		}
	}
}

func processAlive(ctx context.Context, pid int) (bool, error) {
	output, err := exec.CommandContext(ctx, "ps", "-o", "pid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("ps: %w", err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}
