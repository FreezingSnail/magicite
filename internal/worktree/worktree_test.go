package worktree

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestNewDefaultsAndRejectsEscapingWorkspace(t *testing.T) {
	manager, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if manager.workspacePath != defaultWorkspacePath {
		t.Errorf("workspace path = %q, want %q", manager.workspacePath, defaultWorkspacePath)
	}
	if manager.runner == nil {
		t.Error("runner is nil")
	}

	for _, path := range []string{" ", "/workspaces", "..", filepath.Join("..", "workspaces")} {
		if _, err := New(Options{WorkspacePath: path}); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("New(%q) error = %v, want ErrInvalidOptions", path, err)
		}
	}
}

func TestManagerGitSeamPreservesDirectoryAndArgv(t *testing.T) {
	fake := newFakeRunner(fakeReply{Prefix: []string{"status"}, Exit: 7, Output: "dirty"})
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}
	repo := fakeRepo{root: "repo-root"}

	exit, output, err := manager.git(context.Background(), repo, "status", "--short")
	if exit != 7 || output != "dirty" || err != nil {
		t.Errorf("git() = (%d, %q, %v), want (7, dirty, nil)", exit, output, err)
	}
	_, _, _ = manager.gitAt(context.Background(), "other-root", "status")
	calls := fake.Calls()
	if len(calls) != 2 || calls[0].Dir != "repo-root" || calls[1].Dir != "other-root" {
		t.Fatalf("calls = %#v, want roots passed directly", calls)
	}
	if got, want := calls[0].Args, []string{"status", "--short"}; !sameStrings(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestManagerWarnfEmitsOneEvent(t *testing.T) {
	var events []struct {
		level  logging.Level
		kind   string
		fields map[string]any
	}
	manager, err := New(Options{Log: func(level logging.Level, kind string, fields map[string]any) {
		events = append(events, struct {
			level  logging.Level
			kind   string
			fields map[string]any
		}{level, kind, fields})
	}})
	if err != nil {
		t.Fatal(err)
	}
	manager.warnf("seat %s", "ifrit")
	if len(events) != 1 || events[0].level != logging.Warn || events[0].kind != logging.KindWarn || events[0].fields["msg"] != "seat ifrit" {
		t.Errorf("events = %#v, want one warning", events)
	}
}

func TestExecRunnerReturnsExitFailureAsData(t *testing.T) {
	repo := initRepo(t)
	exit, output, err := ExecRunner().Git(context.Background(), repo.Root(), "rev-parse", "--verify", "missing")
	if exit == 0 {
		t.Errorf("exit = 0, want non-zero; output %q", output)
	}
	if err != nil {
		t.Errorf("error = %v, want nil for git exit", err)
	}
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
