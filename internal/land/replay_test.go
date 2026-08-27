package land

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/FreezingSnail/magicite/internal/stamp"
)

type messageRunner struct {
	fake     *fakeRunner
	messages []string
}

func (r *messageRunner) Git(ctx context.Context, dir string, args ...string) (int, string, error) {
	for index, arg := range args {
		if arg == "-F" && index+1 < len(args) {
			content, err := os.ReadFile(args[index+1])
			if err != nil {
				return -1, "", err
			}
			r.messages = append(r.messages, string(content))
		}
	}
	return r.fake.Git(ctx, dir, args...)
}

func TestStampRangeEmpty(t *testing.T) {
	c := replayContext(t)
	fake := newFakeRunner(fakeReply{Prefix: replayArgs(c, "rev-list"), Output: "\n"})
	pipeline := replayPipeline(t, fake, nil)

	if err := pipeline.stampRange(context.Background(), c, replayStamp()); err != nil {
		t.Fatal(err)
	}
	assertReplayArgs(t, fake.Calls(), [][]string{replayArgs(c, "rev-list", "--reverse", "main..ifrit")})
}

func TestStampRangeAmendsSingleCommitFromMessageFile(t *testing.T) {
	c := replayContext(t)
	fake := newFakeRunner(
		fakeReply{Prefix: replayArgs(c, "rev-list"), Output: "one\n"},
		fakeReply{Prefix: replayArgs(c, "rev-parse"), Output: "tip\n"},
		fakeReply{Prefix: replayArgs(c, "log"), Output: "subject\n"},
		fakeReply{Prefix: replayArgs(c, "commit", "--amend")},
	)
	runner := &messageRunner{fake: fake}
	pipeline := replayPipeline(t, runner, nil)

	if err := pipeline.stampRange(context.Background(), c, replayStamp()); err != nil {
		t.Fatal(err)
	}
	assertReplayArgs(t, fake.Calls()[:3], [][]string{
		replayArgs(c, "rev-list", "--reverse", "main..ifrit"),
		replayArgs(c, "rev-parse", "ifrit"),
		replayArgs(c, "log", "-1", "--format=%B", "ifrit"),
	})
	call := fake.Calls()[3].Args
	if len(call) != 6 || !slices.Equal(call[:5], replayArgs(c, "commit", "--amend", "-F")) {
		t.Errorf("amend args = %q, want argv-only commit --amend -F PATH", call)
	}
	if !slices.Equal(runner.messages, []string{"subject\n\nMagicite-Task: task\n"}) {
		t.Errorf("message files = %q, want stamped message", runner.messages)
	}
}

func TestStampRangeReplaysCommitsWithOriginalIdentity(t *testing.T) {
	c := replayContext(t)
	identity := "Ada Lovelace\x00ada@example.test\x002026-08-27T12:00:00-04:00\n"
	fake := newFakeRunner(
		fakeReply{Prefix: replayArgs(c, "rev-list"), Output: "one\ntwo\n"},
		fakeReply{Prefix: replayArgs(c, "rev-parse"), Output: "tip\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%B", "one"), Output: "first\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "one"), Output: identity},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%B", "two"), Output: "second\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "two"), Output: identity},
		fakeReply{Prefix: replayArgs(c, "checkout", "--detach", "main")},
		fakeReply{Prefix: replayArgs(c, "cherry-pick", "--no-commit", "one")},
		fakeReply{Prefix: replayArgs(c, "commit", "-F")},
		fakeReply{Prefix: replayArgs(c, "cherry-pick", "--no-commit", "two")},
		fakeReply{Prefix: replayArgs(c, "branch", "-f", "ifrit", "HEAD")},
		fakeReply{Prefix: replayArgs(c, "checkout", "ifrit")},
	)
	runner := &messageRunner{fake: fake}
	pipeline := replayPipeline(t, runner, nil)

	if err := pipeline.stampRange(context.Background(), c, replayStamp()); err != nil {
		t.Fatal(err)
	}
	calls := fake.Calls()
	if len(calls) != 13 {
		t.Fatalf("calls = %q, want 13", calls)
	}
	assertReplayArgs(t, calls[:8], [][]string{
		replayArgs(c, "rev-list", "--reverse", "main..ifrit"),
		replayArgs(c, "rev-parse", "ifrit"),
		replayArgs(c, "log", "-1", "--format=%B", "one"),
		replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "one"),
		replayArgs(c, "log", "-1", "--format=%B", "two"),
		replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "two"),
		replayArgs(c, "checkout", "--detach", "main"),
		replayArgs(c, "cherry-pick", "--no-commit", "one"),
	})
	for _, index := range []int{8, 10} {
		args := calls[index].Args
		if len(args) != 7 || !slices.Equal(args[:4], replayArgs(c, "commit", "-F")) || args[5] != "--author=Ada Lovelace <ada@example.test>" || args[6] != "--date=2026-08-27T12:00:00-04:00" {
			t.Errorf("commit args = %q, want -F PATH preserved author/date", args)
		}
	}
	assertReplayArgs(t, []fakeCall{calls[9], calls[11], calls[12]}, [][]string{
		replayArgs(c, "cherry-pick", "--no-commit", "two"),
		replayArgs(c, "branch", "-f", "ifrit", "HEAD"),
		replayArgs(c, "checkout", "ifrit"),
	})
	if !slices.Equal(runner.messages, []string{"first\n\nMagicite-Task: task\n", "second\n\nMagicite-Task: task\n"}) {
		t.Errorf("message files = %q, want stamped messages", runner.messages)
	}
}

func TestStampRangeConflictRestoresBranchAndLogsOnce(t *testing.T) {
	c := replayContext(t)
	identity := "Ada\x00ada@example.test\x002026-08-27T12:00:00-04:00\n"
	warnings := 0
	fake := newFakeRunner(
		fakeReply{Prefix: replayArgs(c, "rev-list"), Output: "one\ntwo\n"},
		fakeReply{Prefix: replayArgs(c, "rev-parse"), Output: "tip\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%B", "one"), Output: "first\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "one"), Output: identity},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%B", "two"), Output: "second\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "two"), Output: identity},
		fakeReply{Prefix: replayArgs(c, "checkout", "--detach", "main")},
		fakeReply{Prefix: replayArgs(c, "cherry-pick", "--no-commit", "one"), Code: 1, Output: "CONFLICT (content)"},
		fakeReply{Prefix: replayArgs(c, "cherry-pick", "--abort")},
		fakeReply{Prefix: replayArgs(c, "checkout", "ifrit")},
		fakeReply{Prefix: replayArgs(c, "branch", "-f", "ifrit", "tip")},
	)
	pipeline := replayPipeline(t, fake, &warnings)

	err := pipeline.stampRange(context.Background(), c, replayStamp())
	if !errors.Is(err, ErrConflict) {
		t.Errorf("stampRange() error = %v, want ErrConflict", err)
	}
	if warnings != 1 {
		t.Errorf("warnings = %d, want 1", warnings)
	}
	assertReplayArgs(t, fake.Calls()[8:], [][]string{
		replayArgs(c, "cherry-pick", "--abort"),
		replayArgs(c, "checkout", "ifrit"),
		replayArgs(c, "branch", "-f", "ifrit", "tip"),
	})
}

func TestStampRangeSkipsCanonicalRange(t *testing.T) {
	c := replayContext(t)
	message := "subject\n\nMagicite-Task: task\n"
	fake := newFakeRunner(
		fakeReply{Prefix: replayArgs(c, "rev-list"), Output: "one\ntwo\n"},
		fakeReply{Prefix: replayArgs(c, "rev-parse"), Output: "tip\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%B", "one"), Output: message},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "one"), Output: "Ada\x00ada@example.test\x002026-08-27T12:00:00-04:00\n"},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%B", "two"), Output: message},
		fakeReply{Prefix: replayArgs(c, "log", "-1", "--format=%an%x00%ae%x00%aI", "two"), Output: "Ada\x00ada@example.test\x002026-08-27T12:00:00-04:00\n"},
	)
	pipeline := replayPipeline(t, fake, nil)

	if err := pipeline.stampRange(context.Background(), c, replayStamp()); err != nil {
		t.Fatal(err)
	}
	if len(fake.Calls()) != 6 {
		t.Errorf("calls = %q, want queries only", fake.Calls())
	}
}

func replayContext(t *testing.T) *Context {
	t.Helper()
	return &Context{Root: t.TempDir(), Worktree: "/seat", Branch: "ifrit", Integration: "main"}
}

func replayStamp() stamp.Stamp { return stamp.Stamp{Task: "task"} }

func replayPipeline(t *testing.T, runner Runner, warnings *int) *Pipeline {
	t.Helper()
	log := func(string, string) {}
	if warnings != nil {
		log = func(string, string) { *warnings++ }
	}
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: runner, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	return pipeline
}

func replayArgs(c *Context, args ...string) []string {
	return append([]string{"-C", c.Worktree}, args...)
}

func assertReplayArgs(t *testing.T, calls []fakeCall, want [][]string) {
	t.Helper()
	if len(calls) != len(want) {
		t.Fatalf("calls = %q, want %d", calls, len(want))
	}
	for index, expected := range want {
		if !slices.Equal(calls[index].Args, expected) {
			t.Errorf("call %d = %q, want %q", index, calls[index].Args, expected)
		}
	}
}
