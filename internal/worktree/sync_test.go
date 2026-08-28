package worktree

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
)

func TestSyncOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		replies    func(string) []fakeReply
		wantResult SyncResult
		wantErr    bool
		warnings   int
		want       [][]string
	}{
		{
			name: "already on base", wantResult: SyncSynced,
			replies: func(_ string) []fakeReply {
				return []fakeReply{{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}}}
			},
			want: [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", "main", "ifrit"}},
		},
		{
			name: "dirty stays stale", wantResult: SyncDirty, warnings: 1,
			replies: func(_ string) []fakeReply {
				return []fakeReply{
					{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}, Exit: 1},
					{Prefix: []string{"status", "--porcelain"}, Output: " M file\n"},
				}
			},
			want: [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", "main", "ifrit"}, {"status", "--porcelain"}},
		},
		{
			name: "reset unchanged seat", wantResult: SyncSynced,
			replies: func(_ string) []fakeReply {
				return []fakeReply{
					{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}, Exit: 1},
					{Prefix: []string{"status", "--porcelain"}},
					{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}},
					{Prefix: []string{"reset", "--hard", "main"}},
				}
			},
			want: [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", "main", "ifrit"}, {"status", "--porcelain"}, {"merge-base", "--is-ancestor", "ifrit", "main"}, {"reset", "--hard", "main"}},
		},
		{
			name: "rebase unlanded seat", wantResult: SyncSynced,
			replies: func(_ string) []fakeReply {
				return []fakeReply{
					{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}, Exit: 1},
					{Prefix: []string{"status", "--porcelain"}},
					{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Exit: 1},
					{Prefix: []string{"rebase", "main", "ifrit"}},
				}
			},
			want: [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", "main", "ifrit"}, {"status", "--porcelain"}, {"merge-base", "--is-ancestor", "ifrit", "main"}, {"rebase", "main", "ifrit"}},
		},
		{
			name: "conflict aborts", wantResult: SyncConflict, warnings: 1,
			replies: func(_ string) []fakeReply {
				return failedRebaseReplies("CONFLICT (content): file")
			},
			want: [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", "main", "ifrit"}, {"status", "--porcelain"}, {"merge-base", "--is-ancestor", "ifrit", "main"}, {"rebase", "main", "ifrit"}, {"rebase", "--abort"}},
		},
		{
			name: "failed rebase aborts", wantResult: SyncFailed, wantErr: true, warnings: 1,
			replies: func(_ string) []fakeReply {
				return failedRebaseReplies("bad object")
			},
			want: [][]string{{"worktree", "list", "--porcelain"}, {"merge-base", "--is-ancestor", "main", "ifrit"}, {"status", "--porcelain"}, {"merge-base", "--is-ancestor", "ifrit", "main"}, {"rebase", "main", "ifrit"}, {"rebase", "--abort"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t)
			path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
			warnings, messages := 0, []string{}
			fake := newFakeRunner(append(registeredReplies(path), test.replies(path)...)...)
			manager := syncManager(t, fake, &warnings, &messages)

			got, err := manager.Sync(context.Background(), repo, "ifrit")
			if got != test.wantResult || (err != nil) != test.wantErr {
				t.Fatalf("Sync() = (%s, %v), want (%s, error=%v)", got, err, test.wantResult, test.wantErr)
			}
			if test.wantErr && !errors.Is(err, ErrSyncFailed) {
				t.Errorf("error = %v, want ErrSyncFailed", err)
			}
			assertSyncCalls(t, fake.Calls(), repo.root, path, test.want)
			if warnings != test.warnings {
				t.Errorf("warnings = %d, want %d", warnings, test.warnings)
			}
			if test.wantResult == SyncDirty && !strings.Contains(strings.Join(messages, " "), "fixture seat ifrit stays on a stale base") {
				t.Errorf("dirty warnings = %q, want repo, seat, stale base", messages)
			}
		})
	}
}

func TestSyncRejectsMissingAndUnresolvableSeats(t *testing.T) {
	t.Run("missing registration", func(t *testing.T) {
		repo := initRepo(t)
		warnings := 0
		fake := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + repo.root + "\nbranch refs/heads/main\n"})
		manager := syncManager(t, fake, &warnings, nil)

		result, err := manager.Sync(context.Background(), repo, "ifrit")
		if result != SyncFailed || !errors.Is(err, ErrMissingWorktree) {
			t.Fatalf("Sync() = (%s, %v), want missing worktree failure", result, err)
		}
		if len(fake.Calls()) != 1 || warnings != 1 {
			t.Errorf("calls = %#v, warnings = %d; want registration check and one warning", fake.Calls(), warnings)
		}
	})

	for _, seat := range []string{"", "main"} {
		t.Run("resolve "+seat, func(t *testing.T) {
			repo := initRepo(t)
			fake := newFakeRunner()
			manager := syncManager(t, fake, new(int), nil)

			result, err := manager.Sync(context.Background(), repo, seat)
			if result != SyncFailed || err == nil {
				t.Fatalf("Sync() = (%s, %v), want resolution failure", result, err)
			}
			if len(fake.Calls()) != 0 {
				t.Errorf("git calls = %#v, want none", fake.Calls())
			}
		})
	}
}

func TestSyncAncestorFailuresDoNotWrite(t *testing.T) {
	tests := []struct {
		name    string
		replies []fakeReply
		calls   int
	}{
		{name: "base check", replies: []fakeReply{{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}, Exit: 2, Output: "bad base"}}, calls: 2},
		{name: "seat check", replies: []fakeReply{{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}, Exit: 1}, {Prefix: []string{"status", "--porcelain"}}, {Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Exit: 2, Output: "bad seat"}}, calls: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := initRepo(t)
			path := filepath.Join(repo.root, "harness", "workspaces", "ifrit")
			warnings := 0
			fake := newFakeRunner(append(registeredReplies(path), test.replies...)...)
			manager := syncManager(t, fake, &warnings, nil)

			result, err := manager.Sync(context.Background(), repo, "ifrit")
			if result != SyncFailed || !errors.Is(err, ErrSyncFailed) {
				t.Fatalf("Sync() = (%s, %v), want ancestor failure", result, err)
			}
			if len(fake.Calls()) != test.calls || warnings != 1 {
				t.Errorf("calls = %#v, warnings = %d; want %d predicate calls and one warning", fake.Calls(), warnings, test.calls)
			}
			for _, call := range fake.Calls() {
				if call.Args[0] == "reset" || call.Args[0] == "rebase" {
					t.Errorf("write call = %#v, want none", call)
				}
			}
		})
	}
}

func failedRebaseReplies(output string) []fakeReply {
	return []fakeReply{
		{Prefix: []string{"merge-base", "--is-ancestor", "main", "ifrit"}, Exit: 1},
		{Prefix: []string{"status", "--porcelain"}},
		{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Exit: 1},
		{Prefix: []string{"rebase", "main", "ifrit"}, Exit: 1, Output: output},
		{Prefix: []string{"rebase", "--abort"}},
	}
}

func registeredReplies(path string) []fakeReply {
	return []fakeReply{{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree /repo\nbranch refs/heads/main\n\nworktree " + path + "\nbranch refs/heads/ifrit\n"}}
}

func syncManager(t *testing.T, fake *fakeRunner, warnings *int, messages *[]string) *Manager {
	t.Helper()
	manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, fields map[string]any) {
		if level == logging.Warn {
			*warnings++
			if messages != nil {
				*messages = append(*messages, fields["msg"].(string))
			}
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func assertSyncCalls(t *testing.T, calls []fakeCall, repoPath, seatPath string, want [][]string) {
	t.Helper()
	if len(calls) != len(want) {
		t.Fatalf("git calls = %#v, want %d", calls, len(want))
	}
	for i, args := range want {
		if !sameStrings(calls[i].Args, args) {
			t.Errorf("call %d args = %q, want %q", i, calls[i].Args, args)
		}
		wantDir := repoPath
		if args[0] == "status" || args[0] == "reset" || args[0] == "rebase" {
			wantDir = seatPath
		}
		if calls[i].Dir != wantDir {
			t.Errorf("call %d dir = %q, want %q", i, calls[i].Dir, wantDir)
		}
	}
}
