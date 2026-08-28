package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/client"
	"github.com/connorfranc/magicite/internal/gate"
	"github.com/connorfranc/magicite/internal/server"
	"github.com/connorfranc/magicite/internal/testenv"
	"github.com/connorfranc/magicite/internal/wire"
)

func TestAssembleServesLifecycleOverProcessDoubles(t *testing.T) {
	env := daemonEnv(t)
	record := testenv.NewRepo(t, env, "project")
	if err := os.MkdirAll(filepath.Join(record.Root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	record.Commit("accept gate", map[string]string{"Makefile": "check:\n\t@true\n"})
	prepareSeatCommit(t, record.Root)
	before := record.Head("main")
	beads := testenv.NewBD(t, env)
	testenv.NewAgent(t, env, "kiro")
	applyFixtureEnv(t, env)
	installKiroAlias(t, env)
	installAgentConfig(t, env, "worker", "reviewer")
	beads.Seed(
		testenv.Bead{ID: "epic-1", Title: "epic", Design: "finish the task", AcceptanceCriteria: "task lands", Status: "open", Priority: 1, IssueType: "epic", Labels: []string{}},
		testenv.Bead{ID: "task-1", Title: "task", Description: "implement", Status: "open", Priority: 1, IssueType: "task", Parent: "epic-1", Labels: []string{"difficulty:high"}},
	)
	cfgPath := writeDaemonConfig(t, env, record.Root, "workspaces")
	assembly, err := Assemble(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(ctx, server.Deps{Router: assembly.Router, Bus: assembly.Bus, Socket: assembly.Socket})
	}()
	api := client.New(client.Options{Socket: assembly.Socket, Timeout: 5 * time.Second})
	defer func() {
		var result wire.StopResult
		_ = api.Call(context.Background(), "stop", wire.StopParams{Hard: true}, &result)
		stopDaemon(t, cancel, serveDone, assembly.Socket)
	}()
	waitClient(t, serveDone, api, "status", nil, &wire.StatusResult{})
	var ready []wire.TaskResult
	if err := api.Call(context.Background(), "tasks", wire.TasksParams{Repo: "project"}, &ready); err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "task-1" {
		t.Fatalf("ready tasks = %#v", ready)
	}
	var started wire.StatusResult
	if err := api.Call(context.Background(), "start", nil, &started); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !started.Running {
		t.Fatal("start result is not running")
	}

	waitLifecycle(t, api, beads)
	var events []wire.Event
	if _, err := api.Stream(context.Background(), 0, func(event wire.Event) error {
		events = append(events, event)
		return nil
	}, false); err != nil {
		t.Fatalf("events: %v", err)
	}
	assertLifecycleEvents(t, events)
	if got := record.Head("main"); got == before {
		t.Fatal("task did not land on main")
	}
	task, ok := beads.Bead("task-1")
	if !ok || task.Status != "closed" || !strings.Contains(task.CloseReason, "Magicite-Task: task-1") {
		t.Fatalf("task result = %#v, found = %t", task, ok)
	}
	epic, ok := beads.Bead("epic-1")
	if !ok || len(epic.Comments) == 0 || epic.Comments[len(epic.Comments)-1] != "review ended without a verdict marker." {
		t.Fatalf("review verdict did not reach epic: %#v, found = %t", epic, ok)
	}
	var stopped wire.StopResult
	if err := api.Call(context.Background(), "stop", wire.StopParams{}, &stopped); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if stopped.Mode != "drain" || stopped.Sessions != 0 {
		t.Fatalf("stop result = %#v", stopped)
	}
}

func TestAssembleFailurePathsLeaveNoDaemon(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*testing.T, *testenv.Env) error
	}{
		{
			name: "unreadable config",
			run: func(t *testing.T, _ *testenv.Env) error {
				_, err := Assemble(context.Background(), t.TempDir())
				return err
			},
		},
		{
			name: "unbuildable bd client",
			run: func(t *testing.T, env *testenv.Env) error {
				record := testenv.NewRepo(t, env, "project")
				if err := os.MkdirAll(filepath.Join(record.Root, ".beads"), 0o755); err != nil {
					t.Fatal(err)
				}
				git, err := exec.LookPath("git")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(git, filepath.Join(env.BinDir, "git")); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", env.BinDir)
				path := writeDaemonConfig(t, env, record.Root, "workspaces")
				_, err = Assemble(context.Background(), path)
				return err
			},
		},
		{
			name: "unresolvable workspace",
			run: func(t *testing.T, env *testenv.Env) error {
				record := testenv.NewRepo(t, env, "project")
				path := writeDaemonConfig(t, env, record.Root, "../outside")
				_, err := Assemble(context.Background(), path)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := daemonEnv(t)
			err := test.run(t, env)
			if err == nil || !strings.HasPrefix(err.Error(), "daemon: ") {
				t.Fatalf("Assemble() error = %v, want wrapped daemon error", err)
			}
			if _, statErr := os.Lstat(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "magicite", "magicite.sock")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("daemon socket stat error = %v, want not exist", statErr)
			}
		})
	}
}

func TestAssembleWiresGateAndState(t *testing.T) {
	assembly := assembleForTest(t, true)
	if assembly.State == nil {
		t.Fatal("Assemble() State = nil")
	}
	capability, ok := assembly.Core.(*core)
	if !ok {
		t.Fatalf("Assemble() Core = %T, want *core", assembly.Core)
	}
	qualityGate, ok := capability.gate.(*gate.Gate)
	if !ok {
		t.Fatalf("core gate = %T, want *gate.Gate", capability.gate)
	}
	if !qualityGate.Enabled() {
		t.Fatal("gate unexpectedly disabled")
	}
}

func TestAssembleWiresDisabledGate(t *testing.T) {
	assembly := assembleForTest(t, false)
	capability := assembly.Core.(*core)
	qualityGate, ok := capability.gate.(*gate.Gate)
	if !ok {
		t.Fatalf("core gate = %T, want *gate.Gate", capability.gate)
	}
	if qualityGate.Enabled() {
		t.Fatal("gate unexpectedly enabled")
	}
	if hold, err := qualityGate.Hold(context.Background(), testRepo(t)); err != nil || hold {
		t.Fatalf("disabled gate Hold() = %t, %v", hold, err)
	}
}

func assembleForTest(t *testing.T, enabled bool) *Assembly {
	t.Helper()
	env := daemonEnv(t)
	testenv.NewBD(t, env)
	record := testenv.NewRepo(t, env, "project")
	cfgPath := writeDaemonConfig(t, env, record.Root, "workspaces")
	contents, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	contents = []byte(strings.Replace(string(contents), "enabled: true", fmt.Sprintf("enabled: %t", enabled), 1))
	if err := os.WriteFile(cfgPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	assembly, err := Assemble(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	return assembly
}

func applyFixtureEnv(t *testing.T, env *testenv.Env) {
	t.Helper()
	for _, value := range env.Env() {
		key, setting, ok := strings.Cut(value, "=")
		if ok && key != "PATH" {
			t.Setenv(key, setting)
		}
	}
}

func daemonEnv(t *testing.T) *testenv.Env {
	t.Helper()
	env := testenv.New(t)
	applyFixtureEnv(t, env)
	t.Setenv("PATH", env.BinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))
	socketRoot := filepath.Join(repoRoot, fmt.Sprintf(".magicite-socket-%d", os.Getpid()))
	if err := os.Symlink(env.Root, socketRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socketRoot) })
	t.Setenv("XDG_RUNTIME_DIR", socketRoot)
	if err := os.Chdir(env.Root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	return env
}

func prepareSeatCommit(t *testing.T, root string) {
	t.Helper()
	worktree := filepath.Join(root, "workspaces", "ifrit")
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "worktree", "add", "-b", "ifrit", worktree).CombinedOutput(); err != nil {
		t.Fatalf("create seat worktree: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(worktree, "task.txt"), []byte("done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", worktree, "add", "task.txt"},
		{"-C", worktree, "commit", "-m", "implement task"},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
}

func installKiroAlias(t *testing.T, env *testenv.Env) {
	t.Helper()
	if err := os.Symlink(env.Bin("kiro"), filepath.Join(env.BinDir, "kiro-cli-chat")); err != nil {
		t.Fatal(err)
	}
}

func installAgentConfig(t *testing.T, env *testenv.Env, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(env.Root, name+".json"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func writeDaemonConfig(t *testing.T, env *testenv.Env, root, workspace string) string {
	t.Helper()
	path := filepath.Join(env.Root, "magicite.yaml")
	config := fmt.Sprintf("crew:\n  backend: kiro\nfleet:\n  agent: worker\n  poll-interval: 1\n  seats:\n    - name: ifrit\n      role: implementer\nreviewer:\n  enabled: true\n  agent: reviewer\n  model: reviewer-model\n  seats:\n    - name: odin\n      role: reviewer\nrepos:\n  roots:\n    - %s\nworkspaces:\n  path: %s\n", root, workspace)
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitClient(t *testing.T, serveDone <-chan error, api *client.Client, command string, params, out any) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		if err := api.Call(context.Background(), command, params, out); err == nil {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("daemon did not answer %s", command)
		case <-time.After(time.Millisecond):
		}
	}
}

func waitLifecycle(t *testing.T, api *client.Client, beads *testenv.BD) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		var status wire.StatusResult
		err := api.Call(context.Background(), "status", nil, &status)
		task, taskOK := beads.Bead("task-1")
		epic, epicOK := beads.Bead("epic-1")
		if err == nil && len(status.Sessions) == 0 && taskOK && task.Status == "closed" && epicOK && len(epic.Comments) > 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("lifecycle incomplete: status=%#v task=%#v epic=%#v err=%v", status, task, epic, err)
		case <-time.After(time.Millisecond):
		}
	}
}

func assertLifecycleEvents(t *testing.T, events []wire.Event) {
	t.Helper()
	seen := make(map[wire.Kind]bool)
	for _, event := range events {
		seen[event.Kind] = true
	}
	for _, kind := range []wire.Kind{wire.KindPickup, wire.KindComplete, wire.KindClose} {
		if !seen[kind] {
			t.Fatalf("events = %#v, missing %q", events, kind)
		}
	}
}

func stopDaemon(t *testing.T, cancel context.CancelFunc, done <-chan error, socket string) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Serve() did not stop after cancellation")
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("socket stat error = %v, want not exist", err)
	}
}
