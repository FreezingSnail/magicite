package worktree

import (
	"context"
	"errors"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
)

func TestSyncResultString(t *testing.T) {
	for result, want := range map[SyncResult]string{
		SyncFailed:   "failed",
		SyncSynced:   "synced",
		SyncDirty:    "dirty",
		SyncConflict: "conflict",
		99:           "unknown",
	} {
		if got := result.String(); got != want {
			t.Errorf("SyncResult(%d).String() = %q, want %q", result, got, want)
		}
	}
}

func TestDirty(t *testing.T) {
	tests := []struct {
		name     string
		reply    fakeReply
		want     bool
		warnings int
	}{
		{name: "clean", reply: fakeReply{Prefix: []string{"status", "--porcelain"}}, want: false},
		{name: "status", reply: fakeReply{Prefix: []string{"status", "--porcelain"}, Output: " M file\n"}, want: true},
		{name: "exit", reply: fakeReply{Prefix: []string{"status", "--porcelain"}, Exit: 2, Output: "unreadable"}, want: true, warnings: 1},
		{name: "seam error", reply: fakeReply{Prefix: []string{"status", "--porcelain"}, Err: errors.New("runner failed")}, want: true, warnings: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			warnings := 0
			fake := newFakeRunner(test.reply)
			manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
				if level == logging.Warn {
					warnings++
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			if got := manager.dirty(context.Background(), "/worktree"); got != test.want {
				t.Errorf("dirty() = %v, want %v", got, test.want)
			}
			if len(fake.Calls()) != 1 || !sameStrings(fake.Calls()[0].Args, []string{"status", "--porcelain"}) {
				t.Fatalf("calls = %#v, want one status call", fake.Calls())
			}
			if warnings != test.warnings {
				t.Errorf("warnings = %d, want %d", warnings, test.warnings)
			}
		})
	}
}

func TestAncestor(t *testing.T) {
	repo := fakeRepo{root: "repo-root"}
	tests := []struct {
		name       string
		reply      fakeReply
		ancestor   string
		descendant string
		want       bool
		wantErr    bool
		warnings   int
	}{
		{name: "yes", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "a", "d"}}, ancestor: "a", descendant: "d", want: true},
		{name: "no", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "a", "d"}, Exit: 1, Output: "not ancestor"}, ancestor: "a", descendant: "d"},
		{name: "failure", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "a", "d"}, Exit: 2, Output: "bad commit"}, ancestor: "a", descendant: "d", wantErr: true, warnings: 1},
		{name: "seam error", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "a", "d"}, Err: errors.New("runner failed")}, ancestor: "a", descendant: "d", wantErr: true, warnings: 1},
		{name: "missing ancestor", ancestor: "", descendant: "d", wantErr: true},
		{name: "missing descendant", ancestor: "a", descendant: " ", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			warnings := 0
			fake := newFakeRunner(test.reply)
			manager, err := New(Options{Runner: fake, Log: func(level logging.Level, _ string, _ map[string]any) {
				if level == logging.Warn {
					warnings++
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := manager.ancestor(context.Background(), repo, test.ancestor, test.descendant)
			if got != test.want || (err != nil) != test.wantErr {
				t.Errorf("ancestor() = (%v, %v), want (%v, error=%v)", got, err, test.want, test.wantErr)
			}
			if test.wantErr && test.ancestor != "" && test.descendant != " " && !errors.Is(err, ErrSyncFailed) {
				t.Errorf("error = %v, want ErrSyncFailed", err)
			}
			if len(fake.Calls()) != boolToInt(test.ancestor != "" && test.descendant != " ") {
				t.Fatalf("calls = %#v, want predicate call count", fake.Calls())
			}
			if warnings != test.warnings {
				t.Errorf("warnings = %d, want %d", warnings, test.warnings)
			}
		})
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
