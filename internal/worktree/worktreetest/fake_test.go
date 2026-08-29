package worktreetest

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/FreezingSnail/magicite/internal/worktree"
)

type testRepo struct {
	name string
	root string
}

func (r testRepo) Name() string      { return r.name }
func (r testRepo) Root() string      { return r.root }
func (testRepo) Integration() string { return "main" }

func TestFakeScriptsResultsAndRecordsCalls(t *testing.T) {
	ensureErr := errors.New("ensure")
	syncErr := errors.New("sync")
	cleanupErr := errors.New("cleanup")
	fake := New(map[string]Seat{
		"ifrit": {Path: "/scripted/ifrit", Result: worktree.SyncDirty, EnsureErr: ensureErr, SyncErr: syncErr, CleanupErr: cleanupErr},
	})
	repo := testRepo{name: "magicite", root: "/repo"}
	ctx := context.Background()

	if path, err := fake.Path(repo, "ifrit"); err != nil || path != filepath.Join("/repo", "harness/workspaces", "ifrit") {
		t.Errorf("Path() = %q, %v", path, err)
	}
	if branch, err := fake.Branch(repo, "ifrit"); err != nil || branch != "ifrit" {
		t.Errorf("Branch() = %q, %v", branch, err)
	}
	if path, err := fake.Ensure(ctx, repo, "ifrit"); path != "/scripted/ifrit" || !errors.Is(err, ensureErr) {
		t.Errorf("Ensure() = %q, %v", path, err)
	}
	if result, err := fake.Sync(ctx, repo, "ifrit"); result != worktree.SyncDirty || !errors.Is(err, syncErr) {
		t.Errorf("Sync() = %v, %v", result, err)
	}
	if err := fake.Cleanup(ctx, repo, "ifrit"); !errors.Is(err, cleanupErr) {
		t.Errorf("Cleanup() = %v", err)
	}

	want := []Call{{Op: "path", Repo: "magicite", Seat: "ifrit"}, {Op: "branch", Repo: "magicite", Seat: "ifrit"}, {Op: "ensure", Repo: "magicite", Seat: "ifrit"}, {Op: "sync", Repo: "magicite", Seat: "ifrit"}, {Op: "cleanup", Repo: "magicite", Seat: "ifrit"}}
	if got := fake.Calls(); !sameCalls(got, want) {
		t.Errorf("Calls() = %#v, want %#v", got, want)
	}
}

func TestFakeDefaultsWorkspaceAndSeat(t *testing.T) {
	fake := New(nil)
	fake.WorkspacePath = "custom/workspaces"
	repo := testRepo{name: "magicite", root: "/repo"}

	if path, err := fake.Ensure(context.Background(), repo, "shiva"); err != nil || path != "/repo/custom/workspaces/shiva" {
		t.Errorf("Ensure() = %q, %v", path, err)
	}
	if result, err := fake.Sync(context.Background(), repo, "shiva"); result != worktree.SyncSynced || err != nil {
		t.Errorf("Sync() = %v, %v", result, err)
	}
	if err := fake.Cleanup(context.Background(), repo, "shiva"); err != nil {
		t.Errorf("Cleanup() = %v", err)
	}
}

func TestFakeKeepsResolutionRefusals(t *testing.T) {
	fake := New(nil)
	repo := testRepo{name: "magicite", root: "/repo"}
	for _, seat := range []string{"", ".", ".."} {
		if _, err := fake.Path(repo, seat); !errors.Is(err, worktree.ErrInvalidSeat) {
			t.Errorf("Path(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
		if _, err := fake.Branch(repo, seat); !errors.Is(err, worktree.ErrInvalidSeat) {
			t.Errorf("Branch(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
		if _, err := fake.Ensure(context.Background(), repo, seat); !errors.Is(err, worktree.ErrInvalidSeat) {
			t.Errorf("Ensure(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
		if _, err := fake.Sync(context.Background(), repo, seat); !errors.Is(err, worktree.ErrInvalidSeat) {
			t.Errorf("Sync(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
		if err := fake.Cleanup(context.Background(), repo, seat); !errors.Is(err, worktree.ErrInvalidSeat) {
			t.Errorf("Cleanup(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
	}
	if _, err := fake.Path(testRepo{}, "ifrit"); !errors.Is(err, worktree.ErrUnresolvedRepo) {
		t.Errorf("Path(empty repo) error = %v, want ErrUnresolvedRepo", err)
	}
}

func TestFakeCallsCopyResetAndConcurrentUse(t *testing.T) {
	fake := New(nil)
	repo := testRepo{name: "magicite", root: "/repo"}
	fake.Path(repo, "ifrit")
	calls := fake.Calls()
	calls[0].Op = "changed"
	if fake.Calls()[0].Op != "path" {
		t.Error("Calls returned shared storage")
	}
	fake.Reset()
	if got := fake.Calls(); len(got) != 0 {
		t.Errorf("Calls after Reset = %#v, want none", got)
	}

	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			for range 100 {
				fake.Path(repo, "ifrit")
				fake.Branch(repo, "ifrit")
				fake.Ensure(context.Background(), repo, "ifrit")
				fake.Sync(context.Background(), repo, "ifrit")
				fake.Cleanup(context.Background(), repo, "ifrit")
				fake.Calls()
				fake.Reset()
			}
		})
	}
	group.Wait()
}

func sameCalls(got, want []Call) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
