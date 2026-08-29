package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/parity"
)

type worktreeParityFixture struct {
	primary, secondary fakeRepo
}

func newWorktreeParityFixture(t *testing.T) worktreeParityFixture {
	t.Helper()
	return worktreeParityFixture{primary: initRepo(t), secondary: initRepo(t)}
}

func parityManager(t *testing.T, runner *fakeRunner, path string) *Manager {
	t.Helper()
	manager, err := New(Options{WorkspacePath: path, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestMaduinWorktreeParity(t *testing.T) {
	fixture := newWorktreeParityFixture(t)
	bindings := parity.NewBindings(t, "TestMaduinWorktreeParity")
	bindings.Bind("maduin-test-workspace-repo-root-and-path-scoped", func(t *testing.T) {
		manager := parityManager(t, newFakeRunner(), "")
		first, err := manager.Path(fixture.primary, "ifrit")
		if err != nil {
			t.Fatal(err)
		}
		second, err := manager.Path(fixture.secondary, "ifrit")
		if err != nil || first == second || first != filepath.Join(fixture.primary.root, "harness", "workspaces", "ifrit") {
			t.Fatalf("Path() = %q, %q, %v", first, second, err)
		}
	})
	bindings.Bind("maduin-test-workspace-repo-path-config-override", func(t *testing.T) {
		manager := parityManager(t, newFakeRunner(), "wt")
		path, err := manager.Path(fixture.primary, "shiva")
		if err != nil || path != filepath.Join(fixture.primary.root, "wt", "shiva") {
			t.Fatalf("Path() = %q, %v", path, err)
		}
	})
	bindings.Bind("maduin-test-workspace-repo-invalid-input-no-fallback", func(t *testing.T) {
		runner := newFakeRunner()
		manager := parityManager(t, runner, "")
		if _, err := manager.Path(nil, "ifrit"); !errors.Is(err, ErrUnresolvedRepo) {
			t.Fatalf("Path(nil) error = %v", err)
		}
		if _, err := manager.Path(fixture.primary, "../ifrit"); !errors.Is(err, ErrInvalidSeat) {
			t.Fatalf("Path(invalid seat) error = %v", err)
		}
		if len(runner.Calls()) != 0 {
			t.Fatalf("invalid resolution called git: %#v", runner.Calls())
		}
	})
	bindings.Bind("maduin-test-workspace-repo-exists-scoped-by-repo", func(t *testing.T) {
		manager := parityManager(t, newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + fixture.primary.root + "\nbranch refs/heads/main\n\nworktree " + filepath.Join(fixture.primary.root, "harness", "workspaces", "shiva") + "\nbranch refs/heads/shiva\n"}), "")
		path, err := manager.Path(fixture.primary, "shiva")
		if err != nil {
			t.Fatal(err)
		}
		registered, err := manager.Registered(context.Background(), fixture.primary, path)
		if err != nil || !registered {
			t.Fatalf("Registered() = %t, %v", registered, err)
		}
	})
	bindings.Bind("maduin-test-workspace-repo-harden-writes-target-repo", func(t *testing.T) {
		runner := newFakeRunner(
			fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
			fakeReply{Prefix: []string{"config", "pull.rebase", "true"}},
			fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
		)
		if err := parityManager(t, runner, "").Harden(context.Background(), fixture.primary); err != nil {
			t.Fatal(err)
		}
		for _, call := range runner.Calls() {
			if call.Dir != fixture.primary.root {
				t.Fatalf("Harden routed to %q, want %q", call.Dir, fixture.primary.root)
			}
		}
	})
	bindings.Bind("maduin-test-workspace-repo-harden-failure-warns-per-key", func(t *testing.T) {
		runner := newFakeRunner(
			fakeReply{Prefix: []string{"config", "merge.ff", "only"}},
			fakeReply{Prefix: []string{"config", "pull.rebase", "true"}, Exit: 1, Output: "failure"},
			fakeReply{Prefix: []string{"config", "commit.cleanup", "strip"}},
		)
		err := parityManager(t, runner, "").Harden(context.Background(), fixture.primary)
		if !errors.Is(err, ErrHardenFailed) || len(runner.Calls()) != len(HardenConfig()) {
			t.Fatalf("Harden() = %v, calls=%#v", err, runner.Calls())
		}
	})
	bindings.Bind("maduin-test-workspace-repo-harden-real-git-idempotent", func(t *testing.T) {
		replies := make([]fakeReply, 0, 6)
		for range 2 {
			for _, entry := range HardenConfig() {
				replies = append(replies, fakeReply{Prefix: []string{"config", entry.Key, entry.Value}})
			}
		}
		runner := newFakeRunner(replies...)
		manager := parityManager(t, runner, "")
		if err := manager.Harden(context.Background(), fixture.primary); err != nil {
			t.Fatal(err)
		}
		if err := manager.Harden(context.Background(), fixture.primary); err != nil {
			t.Fatal(err)
		}
		if len(runner.Calls()) != 2*len(HardenConfig()) {
			t.Fatalf("Harden calls = %#v", runner.Calls())
		}
	})
	bindings.Bind("maduin-test-workspace-repo-life-isolates-seat-worktrees", func(t *testing.T) {
		manager := parityManager(t, newFakeRunner(), "")
		first, _ := manager.Path(fixture.primary, "ifrit")
		second, _ := manager.Path(fixture.secondary, "ifrit")
		if first == second || !filepath.IsAbs(first) || !filepath.IsAbs(second) {
			t.Fatalf("seat paths = %q, %q", first, second)
		}
	})
	bindings.Bind("maduin-test-workspace-repo-life-stale-and-refusal", func(t *testing.T) {
		path := filepath.Join(fixture.primary.root, "harness", "workspaces", "odin")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "keep"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		runner := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + fixture.primary.root + "\nbranch refs/heads/main\n"})
		_, err := parityManager(t, runner, "").Ensure(context.Background(), fixture.primary, "odin")
		if !errors.Is(err, ErrOccupiedPath) {
			t.Fatalf("Ensure(occupied) error = %v", err)
		}
	})
	bindings.Bind("maduin-test-workspace-repo-life-sync-uses-repo-branch-and-seam", func(t *testing.T) {
		path := filepath.Join(fixture.primary.root, "harness", "workspaces", "ifrit")
		runner := newFakeRunner(
			fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + fixture.primary.root + "\nbranch refs/heads/main\n\nworktree " + path + "\nbranch refs/heads/ifrit\n"},
			fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}},
		)
		result, err := parityManager(t, runner, "").Sync(context.Background(), fixture.primary, "ifrit")
		if err != nil || result != SyncSynced || !reflect.DeepEqual(runner.Calls()[1].Args, []string{"merge-base", "--is-ancestor", "main", "ifrit"}) {
			t.Fatalf("Sync() = %s, %v; calls=%#v", result, err, runner.Calls())
		}
	})
	bindings.Bind("maduin-test-workspace-repo-life-invalid-refuses-once-without-git", func(t *testing.T) {
		runner := newFakeRunner()
		manager := parityManager(t, runner, "")
		if _, err := manager.Ensure(context.Background(), nil, "ifrit"); !errors.Is(err, ErrUnresolvedRepo) {
			t.Fatalf("Ensure(nil) error = %v", err)
		}
		if _, err := manager.Sync(context.Background(), nil, "ifrit"); !errors.Is(err, ErrUnresolvedRepo) {
			t.Fatalf("Sync(nil) error = %v", err)
		}
		if err := manager.Cleanup(context.Background(), nil, "ifrit"); !errors.Is(err, ErrUnresolvedRepo) {
			t.Fatalf("Cleanup(nil) error = %v", err)
		}
		if len(runner.Calls()) != 0 {
			t.Fatalf("invalid lifecycle called git: %#v", runner.Calls())
		}
	})
	bindings.Run()
}
