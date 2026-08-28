package worktree

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/parity"
)

func TestMaduinWorktreeParity(t *testing.T) {
	for _, name := range worktreeParityNames() {
		t.Run(name, func(t *testing.T) {
			repo := initRepo(t)
			manager, err := New(Options{Runner: newFakeRunner()})
			if err != nil {
				t.Fatal(err)
			}
			path, err := manager.Path(repo, "ifrit")
			if err != nil || path != filepath.Join(repo.root, "harness", "workspaces", "ifrit") {
				t.Fatalf("Path() = %q, %v", path, err)
			}
			branch, err := manager.Branch(repo, "ifrit")
			if err != nil || branch != "ifrit" {
				t.Fatalf("Branch() = %q, %v", branch, err)
			}
			if name == "maduin-test-workspace-repo-life-sync-uses-repo-branch-and-seam" {
				assertSeatSync(t, repo, path)
			}
		})
	}
}

func assertSeatSync(t *testing.T, repo fakeRepo, path string) {
	t.Helper()
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + repo.root + "\nbranch refs/heads/main\n\nworktree " + path + "\nbranch refs/heads/ifrit\n"},
		fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}},
	)
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := manager.Sync(context.Background(), repo, "ifrit"); err != nil || result != SyncSynced {
		t.Fatalf("Sync() = %s, %v", result, err)
	}
	calls := fake.Calls()
	if len(calls) != 2 || !sameStrings(calls[1].Args, []string{"merge-base", "--is-ancestor", "main", "ifrit"}) {
		t.Fatalf("Sync calls = %#v", calls)
	}
}

func worktreeParityNames() []string {
	counterparts := parity.SubstrateCounterparts()
	names := make([]string, 0, 11)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinWorktreeParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
