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

func TestParseList(t *testing.T) {
	output := "worktree /repos/main/\nHEAD 123\nbranch refs/heads/main\n\nworktree /repos/seat/\nHEAD 456\ndetached\n\nworktree /repos/bare\nbare\n\nworktree /repos/branchless\nHEAD 789\n"
	entries, err := ParseList(output)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Path: "/repos/main/", Branch: "main", Name: "main"},
		{Path: "/repos/seat/", Name: "seat"},
		{Path: "/repos/bare", Name: "bare"},
		{Path: "/repos/branchless", Name: "branchless"},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Errorf("entry %d = %#v, want %#v", i, entries[i], want[i])
		}
	}
}

func TestParseListEmptyAndMalformed(t *testing.T) {
	entries, err := ParseList("")
	if err != nil || entries == nil || len(entries) != 0 {
		t.Errorf("ParseList(empty) = %#v, %v; want non-nil empty, nil", entries, err)
	}
	for _, output := range []string{"worktree\n", "worktree \n", "branch refs/heads/main\n"} {
		if _, err := ParseList(output); !errors.Is(err, ErrMalformedList) {
			t.Errorf("ParseList(%q) error = %v, want ErrMalformedList", output, err)
		}
	}
}

func TestListCallsGitAndReportsFailures(t *testing.T) {
	repo := fakeRepo{name: "fixture", root: t.TempDir(), integration: "main"}
	t.Run("success", func(t *testing.T) {
		fake := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree /repos/main\nbranch refs/heads/main\n"})
		manager, err := New(Options{Runner: fake})
		if err != nil {
			t.Fatal(err)
		}
		entries, err := manager.List(context.Background(), repo)
		if err != nil || len(entries) != 1 || entries[0].Branch != "main" {
			t.Fatalf("List() = %#v, %v", entries, err)
		}
		calls := fake.Calls()
		if len(calls) != 1 || !sameStrings(calls[0].Args, []string{"worktree", "list", "--porcelain"}) || calls[0].Dir != repo.root {
			t.Errorf("calls = %#v, want one list call in %q", calls, repo.root)
		}
	})
	for _, reply := range []fakeReply{{Exit: 9, Output: "broken"}, {Exit: -1, Output: "offline", Err: errors.New("runner failed")}} {
		t.Run("failure", func(t *testing.T) {
			fake := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Exit: reply.Exit, Output: reply.Output, Err: reply.Err})
			warnings := 0
			manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
				if level == logging.Warn {
					warnings++
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = manager.List(context.Background(), repo)
			if err == nil || !strings.Contains(err.Error(), "exit ") || !strings.Contains(err.Error(), reply.Output) {
				t.Errorf("List() error = %v, want exit and output", err)
			}
			if len(fake.Calls()) != 1 || warnings != 1 {
				t.Errorf("calls = %d, warnings = %d; want one each", len(fake.Calls()), warnings)
			}
		})
	}
}

func TestInfoAndRegisteredCanonicalizeBothPaths(t *testing.T) {
	repo := fakeRepo{name: "fixture", root: t.TempDir(), integration: "main"}
	physical := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(physical, link); err != nil {
		t.Fatal(err)
	}
	output := "worktree " + physical + "\nbranch refs/heads/ifrit\n"

	for _, call := range []string{"info", "registered"} {
		t.Run(call, func(t *testing.T) {
			fake := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: output})
			manager, err := New(Options{Runner: fake})
			if err != nil {
				t.Fatal(err)
			}
			if call == "info" {
				entry, found, err := manager.Info(context.Background(), repo, link+string(filepath.Separator))
				if err != nil || !found || entry.Path != physical {
					t.Errorf("Info() = %#v, %t, %v", entry, found, err)
				}
			} else {
				found, err := manager.Registered(context.Background(), repo, link)
				if err != nil || !found {
					t.Errorf("Registered() = %t, %v", found, err)
				}
			}
			if len(fake.Calls()) != 1 {
				t.Errorf("git calls = %#v, want one", fake.Calls())
			}
		})
	}
}

func TestInfoUnregisteredAndInvalidDoNotMutate(t *testing.T) {
	repo := fakeRepo{name: "fixture", root: t.TempDir(), integration: "main"}
	fake := newFakeRunner(fakeReply{Prefix: []string{"worktree", "list", "--porcelain"}, Output: "worktree " + t.TempDir() + "\n"})
	manager, err := New(Options{Runner: fake})
	if err != nil {
		t.Fatal(err)
	}
	if entry, found, err := manager.Info(context.Background(), repo, filepath.Join(t.TempDir(), "pending")); err != nil || found || entry != (Entry{}) {
		t.Errorf("Info() = %#v, %t, %v; want zero, false, nil", entry, found, err)
	}
	if len(fake.Calls()) != 1 {
		t.Fatalf("git calls = %d, want one", len(fake.Calls()))
	}
	for _, invalid := range []struct {
		repo Repo
		dir  string
	}{{nil, t.TempDir()}, {repo, ""}} {
		found, err := manager.Registered(context.Background(), invalid.repo, invalid.dir)
		if err == nil || found {
			t.Errorf("Registered(%v, %q) = %t, %v; want false, error", invalid.repo, invalid.dir, found, err)
		}
	}
	if len(fake.Calls()) != 1 {
		t.Errorf("invalid registration spawned git: %#v", fake.Calls())
	}
}
