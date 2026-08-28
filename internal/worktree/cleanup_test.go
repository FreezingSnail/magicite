package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
)

func TestCleanupRemovesSeatWorktreeBranchAndPrunes(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"rev-parse", "--verify", "ifrit"}},
		fakeReply{Prefix: []string{"worktree", "remove", "--force", path}},
		fakeReply{Prefix: []string{"branch", "-D", "ifrit"}},
		fakeReply{Prefix: []string{"worktree", "prune"}},
	)
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Cleanup(context.Background(), repo, "ifrit"); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	calls := fake.Calls()
	want := [][]string{
		{"rev-parse", "--verify", "ifrit"},
		{"worktree", "remove", "--force", path},
		{"branch", "-D", "ifrit"},
		{"worktree", "prune"},
	}
	if len(calls) != len(want) {
		t.Fatalf("git call count = %d, want %d: %#v", len(calls), len(want), calls)
	}
	for i := range want {
		if !sameStrings(calls[i].Args, want[i]) || calls[i].Dir != repo.root {
			t.Errorf("call %d = (%q, %q), want (%q, %q)", i, calls[i].Dir, calls[i].Args, repo.root, want[i])
		}
	}
}

func TestCleanupMissingPathAndBranchAreNoops(t *testing.T) {
	repo := initRepo(t)
	warnings := 0
	fake := newFakeRunner(fakeReply{Prefix: []string{"rev-parse", "--verify", "ifrit"}, Exit: 1, Output: "missing"})
	manager, err := New(Options{
		Runner: fake,
		Log: func(level logging.Level, _ string, _ map[string]any) {
			if level == logging.Warn {
				warnings++
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Cleanup(context.Background(), repo, "ifrit"); err != nil {
		t.Fatalf("missing path Cleanup() error = %v", err)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("missing path git calls = %#v, want none", fake.Calls())
	}

	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.Cleanup(context.Background(), repo, "ifrit"); err != nil {
		t.Fatalf("missing branch Cleanup() error = %v", err)
	}
	if len(fake.Calls()) != 1 || warnings != 1 {
		t.Errorf("calls = %#v, warnings = %d; want one verify and one warning", fake.Calls(), warnings)
	}
}

func TestCleanupStopsOnRemovalFailure(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"rev-parse", "--verify", "ifrit"}},
		fakeReply{Prefix: []string{"worktree", "remove", "--force", path}, Exit: 2, Output: "locked"},
	)
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Cleanup(context.Background(), repo, "ifrit")
	if !errors.Is(err, ErrCleanupFailed) || !contains(err.Error(), "exit 2") || !contains(err.Error(), "locked") {
		t.Fatalf("Cleanup() error = %v, want cleanup failure with exit and output", err)
	}
	if len(fake.Calls()) != 2 {
		t.Fatalf("git calls = %#v, want stop after removal", fake.Calls())
	}
}

func TestCleanupBranchFailureAndPruneFailure(t *testing.T) {
	t.Run("branch failure", func(t *testing.T) {
		repo := initRepo(t)
		path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		fake := newFakeRunner(
			fakeReply{Prefix: []string{"rev-parse", "--verify", "ifrit"}},
			fakeReply{Prefix: []string{"worktree", "remove", "--force", path}},
			fakeReply{Prefix: []string{"branch", "-D", "ifrit"}, Exit: 1, Output: "checked out"},
		)
		manager, err := New(Options{Runner: fake})
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.Cleanup(context.Background(), repo, "ifrit"); !errors.Is(err, ErrCleanupFailed) {
			t.Fatalf("Cleanup() error = %v, want cleanup failure", err)
		}
		if len(fake.Calls()) != 3 {
			t.Fatalf("git calls = %#v, want stop before prune", fake.Calls())
		}
	})

	t.Run("prune failure is advisory", func(t *testing.T) {
		repo := initRepo(t)
		path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		fake := newFakeRunner(
			fakeReply{Prefix: []string{"rev-parse", "--verify", "ifrit"}},
			fakeReply{Prefix: []string{"worktree", "remove", "--force", path}},
			fakeReply{Prefix: []string{"branch", "-D", "ifrit"}},
			fakeReply{Prefix: []string{"worktree", "prune"}, Exit: 1, Output: "stale"},
		)
		manager, err := New(Options{Runner: fake})
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.Cleanup(context.Background(), repo, "ifrit"); err != nil {
			t.Fatalf("Cleanup() error = %v, want nil", err)
		}
	})
}

func TestCleanupRefusesProtectedBranchWithoutGit(t *testing.T) {
	repo := initRepo(t)
	fake := newFakeRunner()
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Cleanup(context.Background(), repo, "main")
	if !errors.Is(err, ErrProtectedBranch) {
		t.Fatalf("Cleanup() error = %v, want ErrProtectedBranch", err)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("git calls = %#v, want none", fake.Calls())
	}
}

func contains(s, want string) bool {
	return strings.Contains(s, want)
}
