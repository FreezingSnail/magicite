package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
)

func TestEnsureCreatesMissingSeatAndReusesRegisteredSeat(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	fake := &ensureRunner{lists: []string{
		"worktree " + repo.root + "\nbranch refs/heads/main\n",
		"worktree " + path + "\nbranch refs/heads/ifrit\n",
		"worktree " + path + "\nbranch refs/heads/ifrit\n",
		"worktree " + path + "\nbranch refs/heads/ifrit\n",
	}}
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	got, err := manager.Ensure(context.Background(), repo, "ifrit")
	if err != nil {
		t.Fatalf("Ensure() create error = %v", err)
	}
	if got != path {
		t.Fatalf("Ensure() path = %q, want %q", got, path)
	}

	got, err = manager.Ensure(context.Background(), repo, "ifrit")
	if err != nil {
		t.Fatalf("Ensure() reuse error = %v", err)
	}
	if got != path {
		t.Fatalf("Ensure() reused path = %q, want %q", got, path)
	}
	calls := fake.Calls()
	adds := 0
	for _, call := range calls {
		if len(call.Args) >= 2 && call.Args[0] == "worktree" && call.Args[1] == "add" {
			adds++
		}
	}
	if adds != 1 {
		t.Errorf("worktree add calls = %d, want one; calls = %#v", adds, calls)
	}
}

func TestEnsureRemovesEmptyUnregisteredDirectory(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &ensureRunner{lists: []string{
		"worktree " + repo.root + "\nbranch refs/heads/main\n",
		"worktree " + path + "\nbranch refs/heads/ifrit\n",
	}}
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Ensure(context.Background(), repo, "ifrit"); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("created path stat = %v", err)
	}
}

func TestEnsureRefusesOccupiedPathsAndWarnsOnce(t *testing.T) {
	tests := []struct {
		name     string
		makePath func(t *testing.T, path string)
	}{
		{name: "non-empty directory", makePath: func(t *testing.T, path string) {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "change"), []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "file", makePath: func(t *testing.T, path string) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initRepo(t)
			path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
			tt.makePath(t, path)
			warnings := 0
			fake := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + repo.root + "\nbranch refs/heads/main\n"})
			manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
				if level == logging.Warn {
					warnings++
				}
			}})
			if err != nil {
				t.Fatal(err)
			}

			_, err = manager.Ensure(context.Background(), repo, "ifrit")
			if !errors.Is(err, ErrOccupiedPath) || !contains(err.Error(), path) {
				t.Fatalf("Ensure() error = %v, want occupied path %q", err, path)
			}
			if warnings != 1 {
				t.Errorf("warnings = %d, want one", warnings)
			}
			if _, statErr := os.Stat(path); statErr != nil {
				t.Errorf("occupied path removed: %v", statErr)
			}
			if len(fake.Calls()) != 1 {
				t.Errorf("git calls = %#v, want list only", fake.Calls())
			}
		})
	}
}

func TestEnsureReturnsResolutionAndListErrorsUnchanged(t *testing.T) {
	repo := initRepo(t)
	fake := newFakeRunner()
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Ensure(context.Background(), repo, "../")
	if !errors.Is(err, ErrInvalidSeat) {
		t.Fatalf("Ensure() resolution error = %v, want ErrInvalidSeat", err)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("resolution calls = %#v, want none", fake.Calls())
	}

	listErr := errors.New("list failed")
	fake = newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Exit: -1, Err: listErr})
	manager, err = New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Ensure(context.Background(), repo, "ifrit")
	if !errors.Is(err, listErr) {
		t.Fatalf("Ensure() list error = %v, want %v", err, listErr)
	}
	if len(fake.Calls()) != 1 {
		t.Fatalf("list calls = %#v, want one", fake.Calls())
	}
}

type ensureRunner struct {
	calls     []fakeCall
	lists     []string
	listIndex int
}

func (r *ensureRunner) Git(_ context.Context, dir string, args ...string) (int, string, error) {
	r.calls = append(r.calls, fakeCall{Dir: dir, Args: append([]string(nil), args...)})
	if len(args) == 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain" {
		if r.listIndex >= len(r.lists) {
			return -1, "", fmt.Errorf("unexpected list call %d", r.listIndex)
		}
		output := r.lists[r.listIndex]
		r.listIndex++
		return 0, output, nil
	}
	if len(args) >= 3 && args[0] == "worktree" && args[1] == "add" {
		if err := os.MkdirAll(args[2], 0o755); err != nil {
			return -1, "", err
		}
	}
	return 0, "", nil
}

func (r *ensureRunner) Calls() []fakeCall {
	calls := make([]fakeCall, len(r.calls))
	copy(calls, r.calls)
	return calls
}
