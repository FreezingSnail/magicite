package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestCreateHardensAddsAndReturnsRegisteredPath(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
		fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
		fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
		fakeReply{Prefix: []string{"worktree", "add", path, "-b", "ifrit"}},
		fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + path + "\nbranch refs/heads/ifrit\n"},
	)
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	got, err := manager.Create(context.Background(), repo, "ifrit")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got != path {
		t.Errorf("Create() path = %q, want registered path %q", got, path)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("workspace root missing: %v", err)
	}
	assertCreateCalls(t, fake.Calls(), repo.root, [][]string{
		{"config", "merge.ff", "only"},
		{"config", "pull.rebase", "true"},
		{"config", "commit.cleanup", "strip"},
		{"worktree", "add", path, "-b", "ifrit"},
		{"worktree", "list", "--porcelain"},
	})
}

func TestCreateRetriesAddWithoutBranchCreation(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
		fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
		fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
		fakeReply{Prefix: []string{"worktree", "add", path, "-b", "ifrit"}, Exit: 128, Output: "branch exists"},
		fakeReply{Prefix: []string{"worktree", "add", path, "ifrit"}},
		fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + path + "\nbranch refs/heads/ifrit\n"},
	)
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Create(context.Background(), repo, "ifrit"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	assertCreateCalls(t, fake.Calls(), repo.root, [][]string{
		{"config", "merge.ff", "only"},
		{"config", "pull.rebase", "true"},
		{"config", "commit.cleanup", "strip"},
		{"worktree", "add", path, "-b", "ifrit"},
		{"worktree", "add", path, "ifrit"},
		{"worktree", "list", "--porcelain"},
	})
}

func TestCreateStopsOnHardeningOrAddSeamFailure(t *testing.T) {
	t.Run("hardening", func(t *testing.T) {
		repo := initRepo(t)
		fake := newFakeRunner(
			fakeReply{Prefix: []string{"config", "merge.ff", "only"}, Exit: 1},
			fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
			fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
		)
		manager, err := New(Options{Runner: fake})
		if err != nil {
			t.Fatal(err)
		}

		_, err = manager.Create(context.Background(), repo, "ifrit")
		if !errors.Is(err, ErrHardenFailed) {
			t.Fatalf("Create() error = %v, want ErrHardenFailed", err)
		}
		if len(fake.Calls()) != 3 {
			t.Fatalf("git calls = %#v, want hardening only", fake.Calls())
		}
	})

	t.Run("add seam", func(t *testing.T) {
		repo := initRepo(t)
		path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
		seamErr := errors.New("runner unavailable")
		fake := newFakeRunner(
			fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
			fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
			fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
			fakeReply{Prefix: []string{"worktree", "add", path, "-b", "ifrit"}, Exit: -1, Err: seamErr},
		)
		manager, err := New(Options{Runner: fake})
		if err != nil {
			t.Fatal(err)
		}

		_, err = manager.Create(context.Background(), repo, "ifrit")
		if !errors.Is(err, ErrCreateFailed) || !errors.Is(err, seamErr) {
			t.Fatalf("Create() error = %v, want wrapped create and seam failures", err)
		}
		if len(fake.Calls()) != 4 {
			t.Fatalf("git calls = %#v, want no add retry", fake.Calls())
		}
	})
}

func TestCreateRefusesResolutionWithoutFilesystemOrGit(t *testing.T) {
	repo := initRepo(t)
	fake := newFakeRunner()
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	_, err = manager.Create(context.Background(), repo, repo.integration)
	if !errors.Is(err, ErrProtectedBranch) {
		t.Fatalf("Create() error = %v, want ErrProtectedBranch", err)
	}
	workspace := filepath.Join(repo.root, "harness", "workspaces")
	if _, statErr := os.Stat(workspace); !os.IsNotExist(statErr) {
		t.Errorf("workspace stat = %v, want absent", statErr)
	}
	if len(fake.Calls()) != 0 {
		t.Errorf("resolution spawned git: %#v", fake.Calls())
	}
}

func TestCreateLeavesUnregisteredWorktreeForInspection(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
		fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
		fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
		fakeReply{Prefix: []string{"worktree", "add", path, "-b", "ifrit"}},
		fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + repo.root + "\nbranch refs/heads/main\n"},
	)
	warnings := 0
	manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
		if level == logging.Warn {
			warnings++
		}
	}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = manager.Create(context.Background(), repo, "ifrit")
	if !errors.Is(err, ErrCreateFailed) {
		t.Fatalf("Create() error = %v, want ErrCreateFailed", err)
	}
	if warnings != 1 {
		t.Errorf("warnings = %d, want one", warnings)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("workspace root missing: %v", err)
	}
}

func TestCreateReportsSecondAddFailureAndRootCreationFailure(t *testing.T) {
	t.Run("second add", func(t *testing.T) {
		repo := initRepo(t)
		path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
		fake := newFakeRunner(
			fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
			fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
			fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
			fakeReply{Prefix: []string{"worktree", "add", path, "-b", "ifrit"}, Exit: 128, Output: "branch exists"},
			fakeReply{Prefix: []string{"worktree", "add", path, "ifrit"}, Exit: 1, Output: "checkout failed"},
		)
		warnings := 0
		manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
			if level == logging.Warn {
				warnings++
			}
		}})
		if err != nil {
			t.Fatal(err)
		}

		_, err = manager.Create(context.Background(), repo, "ifrit")
		if !errors.Is(err, ErrCreateFailed) || !contains(err.Error(), "exit 1") || !contains(err.Error(), "checkout failed") {
			t.Fatalf("Create() error = %v, want second add failure", err)
		}
		if len(fake.Calls()) != 5 || warnings != 1 {
			t.Errorf("calls = %#v, warnings = %d; want two adds and one warning", fake.Calls(), warnings)
		}
	})

	t.Run("workspace root", func(t *testing.T) {
		repo := initRepo(t)
		if err := os.WriteFile(filepath.Join(repo.root, "blocked"), []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		fake := newFakeRunner()
		warnings := 0
		manager, err := New(Options{WorkspacePath: "blocked/workspaces", Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
			if level == logging.Warn {
				warnings++
			}
		}})
		if err != nil {
			t.Fatal(err)
		}

		_, err = manager.Create(context.Background(), repo, "ifrit")
		if !errors.Is(err, ErrCreateFailed) {
			t.Fatalf("Create() error = %v, want ErrCreateFailed", err)
		}
		if len(fake.Calls()) != 0 || warnings != 1 {
			t.Errorf("calls = %#v, warnings = %d; want no git and one warning", fake.Calls(), warnings)
		}
	})
}

func assertCreateCalls(t *testing.T, calls []fakeCall, dir string, want [][]string) {
	t.Helper()
	if len(calls) != len(want) {
		t.Fatalf("git calls = %#v, want %d", calls, len(want))
	}
	for i := range want {
		if calls[i].Dir != dir || !sameStrings(calls[i].Args, want[i]) {
			t.Errorf("call %d = (%q, %q), want (%q, %q)", i, calls[i].Dir, calls[i].Args, dir, want[i])
		}
	}
}
