package worktree

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
)

type hardenRunner struct {
	calls   []fakeCall
	replies map[string]fakeReply
}

func (r *hardenRunner) Git(_ context.Context, dir string, args ...string) (int, string, error) {
	r.runs(dir, args...)
	reply, ok := r.replies[strings.Join(args, "\x00")]
	if !ok {
		return -1, "", fmt.Errorf("unexpected git argv: %q", args)
	}
	return reply.Exit, reply.Output, reply.Err
}

func (r *hardenRunner) runs(dir string, args ...string) {
	r.calls = append(r.calls, fakeCall{Dir: dir, Args: append([]string(nil), args...)})
}

func TestHardenConfigIsOrderedAndFresh(t *testing.T) {
	want := []ConfigEntry{
		{Key: "merge.ff", Value: "only"},
		{Key: "pull.rebase", Value: "true"},
		{Key: "commit.cleanup", Value: "strip"},
	}
	got := HardenConfig()
	if !sameConfigEntries(got, want) {
		t.Fatalf("HardenConfig() = %#v, want %#v", got, want)
	}
	got[0].Value = "false"
	if fresh := HardenConfig(); !sameConfigEntries(fresh, want) {
		t.Errorf("HardenConfig() after mutation = %#v, want %#v", fresh, want)
	}
}

func TestHardenAppliesEveryConfigEntryWithoutWarnings(t *testing.T) {
	repo := fakeRepo{name: "fixture", root: t.TempDir(), integration: "main"}
	runner := &hardenRunner{replies: map[string]fakeReply{
		"config\x00merge.ff\x00only":        {},
		"config\x00pull.rebase\x00true":     {},
		"config\x00commit.cleanup\x00strip": {},
	}}
	var warnings []string
	manager, err := New(Options{Runner: runner, Log: func(_ logging.Level, _ string, fields map[string]any) {
		warnings = append(warnings, fields["msg"].(string))
	}})
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Harden(context.Background(), repo); err != nil {
		t.Fatalf("Harden() error = %v", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("git calls = %d, want 3", len(runner.calls))
	}
	wantArgs := [][]string{
		{"config", "merge.ff", "only"},
		{"config", "pull.rebase", "true"},
		{"config", "commit.cleanup", "strip"},
	}
	for i, want := range wantArgs {
		if !sameStrings(runner.calls[i].Args, want) {
			t.Errorf("call %d argv = %q, want %q", i, runner.calls[i].Args, want)
		}
		if runner.calls[i].Dir != repo.root {
			t.Errorf("call %d dir = %q, want %q", i, runner.calls[i].Dir, repo.root)
		}
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %q, want none", warnings)
	}
}

func TestHardenAttemptsAllFailuresAndReportsEachKey(t *testing.T) {
	repo := fakeRepo{name: "fixture", root: t.TempDir(), integration: "main"}
	seamErr := errors.New("runner unavailable")
	runner := &hardenRunner{replies: map[string]fakeReply{
		"config\x00merge.ff\x00only":        {Exit: 3, Output: "rejected"},
		"config\x00pull.rebase\x00true":     {},
		"config\x00commit.cleanup\x00strip": {Exit: -1, Err: seamErr},
	}}
	var warnings []string
	manager, err := New(Options{Runner: runner, Log: func(_ logging.Level, _ string, fields map[string]any) {
		warnings = append(warnings, fields["msg"].(string))
	}})
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Harden(context.Background(), repo)
	if !errors.Is(err, ErrHardenFailed) {
		t.Fatalf("Harden() error = %v, want ErrHardenFailed", err)
	}
	for _, key := range []string{"merge.ff", "commit.cleanup"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("Harden() error = %v, missing failed key %q", err, key)
		}
	}
	if len(runner.calls) != 3 {
		t.Fatalf("git calls = %d, want 3", len(runner.calls))
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %q, want one per failed key", warnings)
	}
	if !strings.Contains(warnings[0], "merge.ff") || !strings.Contains(warnings[0], "only") || !strings.Contains(warnings[0], "exit 3") {
		t.Errorf("first warning = %q, missing key/value/exit", warnings[0])
	}
	if !strings.Contains(warnings[1], "commit.cleanup") || !strings.Contains(warnings[1], "strip") || !strings.Contains(warnings[1], "exit -1") {
		t.Errorf("second warning = %q, missing key/value/exit", warnings[1])
	}
}

func TestHardenResolutionFailureWarnsWithoutGit(t *testing.T) {
	runner := &hardenRunner{replies: map[string]fakeReply{}}
	var warnings []string
	manager, err := New(Options{Runner: runner, Log: func(_ logging.Level, _ string, fields map[string]any) {
		warnings = append(warnings, fields["msg"].(string))
	}})
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Harden(context.Background(), fakeRepo{name: "fixture", root: t.TempDir() + "/missing", integration: "main"})
	if !errors.Is(err, ErrUnresolvedRepo) {
		t.Fatalf("Harden() error = %v, want ErrUnresolvedRepo", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("git calls = %d, want none", len(runner.calls))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %q, want one", warnings)
	}
}

func sameConfigEntries(left, right []ConfigEntry) bool {
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
