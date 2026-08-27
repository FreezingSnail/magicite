package land

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/stamp"
)

type orderedRunner struct {
	replies []fakeReply
	calls   []fakeCall
}

func (r *orderedRunner) Git(_ context.Context, dir string, args ...string) (int, string, error) {
	r.calls = append(r.calls, fakeCall{Dir: dir, Args: append([]string(nil), args...)})
	if len(r.replies) == 0 {
		return -1, "", fmt.Errorf("unexpected git argv: %q", args)
	}
	reply := r.replies[0]
	r.replies = r.replies[1:]
	if len(args) < len(reply.Prefix) || !slices.Equal(args[:len(reply.Prefix)], reply.Prefix) {
		return -1, "", fmt.Errorf("git argv = %q, want prefix %q", args, reply.Prefix)
	}
	return reply.Code, reply.Output, reply.Err
}

func TestFastForwardUsesOnlyFFOnlyAtIntegrationRoot(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{{Prefix: []string{"merge", "--ff-only", "ifrit"}}}}
	pipeline := fastForwardPipeline(t, runner, nil, nil)

	exit, output, err := pipeline.fastForward(context.Background(), c)
	if exit != 0 || output != "" || err != nil {
		t.Fatalf("fastForward() = (%d, %q, %v), want (0, empty, nil)", exit, output, err)
	}
	if len(runner.calls) != 1 || runner.calls[0].Dir != c.Root || !slices.Equal(runner.calls[0].Args, []string{"merge", "--ff-only", "ifrit"}) {
		t.Errorf("calls = %#v, want root merge --ff-only ifrit", runner.calls)
	}
}

func TestDivergedFF(t *testing.T) {
	for _, test := range []struct {
		output string
		want   bool
	}{
		{"fatal: Not Possible To Fast-Forward, aborting.", true},
		{"fatal: diverging branches can't be fast-forwarded", true},
		{"merge failed because hooks rejected it", false},
	} {
		if got := divergedFF(test.output); got != test.want {
			t.Errorf("divergedFF(%q) = %v, want %v", test.output, got, test.want)
		}
	}
}

func TestLandRebasedGateFailureDoesNotFastForward(t *testing.T) {
	c := fastForwardContext()
	var logs []string
	pipeline := fastForwardPipeline(t, &orderedRunner{}, func(context.Context, *Context) (int, error) { return 7, nil }, &logs)

	err := pipeline.landRebased(context.Background(), c, nil)
	if !errors.Is(err, ErrGateFailed) {
		t.Fatalf("landRebased() error = %v, want ErrGateFailed", err)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], c.Branch) {
		t.Errorf("warnings = %q, want one warning naming %q", logs, c.Branch)
	}
}

func TestLandRebasedRefusesNonDivergentFastForwardWithoutRetry(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{{Prefix: []string{"merge", "--ff-only", "ifrit"}, Code: 4, Output: "hook rejected"}}}
	var logs []string
	pipeline := fastForwardPipeline(t, runner, successfulGate, &logs)

	err := pipeline.landRebased(context.Background(), c, nil)
	if err == nil || !strings.Contains(err.Error(), "exit 4") || !strings.Contains(err.Error(), "hook rejected") {
		t.Fatalf("landRebased() error = %v, want fast-forward exit/output", err)
	}
	if len(runner.calls) != 1 {
		t.Errorf("git calls = %q, want one non-retried fast-forward", runner.calls)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], c.Branch) {
		t.Errorf("warnings = %q, want one warning naming %q", logs, c.Branch)
	}
}

func TestLandRebasedRecoversDivergenceOnceWithRestampAndRegate(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"merge", "--ff-only", "ifrit"}, Code: 1, Output: "Not possible to fast-forward"},
		{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}},
		{Prefix: []string{"-C", c.Worktree, "rev-list", "--reverse", "main..ifrit"}, Output: "\n"},
		{Prefix: []string{"merge", "--ff-only", "ifrit"}},
	}}
	gates := 0
	pipeline := fastForwardPipeline(t, runner, func(context.Context, *Context) (int, error) {
		gates++
		return 0, nil
	}, nil)

	if err := pipeline.landRebased(context.Background(), c, &stamp.Stamp{Task: "magicite-ewp.9"}); err != nil {
		t.Fatal(err)
	}
	if gates != 2 {
		t.Errorf("gate calls = %d, want 2", gates)
	}
	assertFastForwardArgs(t, runner.calls, [][]string{
		{"merge", "--ff-only", "ifrit"},
		{"-C", c.Worktree, "rebase", "main", "ifrit"},
		{"-C", c.Worktree, "rev-list", "--reverse", "main..ifrit"},
		{"merge", "--ff-only", "ifrit"},
	})
}

func TestLandRebasedSecondDivergenceRefusesConflict(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"merge", "--ff-only", "ifrit"}, Code: 1, Output: "diverging branches"},
		{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}},
		{Prefix: []string{"merge", "--ff-only", "ifrit"}, Code: 1, Output: "not possible to fast-forward"},
	}}
	var logs []string
	pipeline := fastForwardPipeline(t, runner, successfulGate, &logs)

	err := pipeline.landRebased(context.Background(), c, nil)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("landRebased() error = %v, want ErrConflict", err)
	}
	if len(runner.calls) != 3 {
		t.Errorf("git calls = %q, want exactly two fast-forwards and one rebase", runner.calls)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], c.Branch) {
		t.Errorf("warnings = %q, want one warning naming %q", logs, c.Branch)
	}
}

func TestLandRebasedRebaseConflictRefusesConflict(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"merge", "--ff-only", "ifrit"}, Code: 1, Output: "diverging branches"},
		{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}, Code: 1, Output: "CONFLICT (content)"},
		{Prefix: []string{"-C", c.Worktree, "rebase", "--abort"}},
	}}
	var logs []string
	pipeline := fastForwardPipeline(t, runner, successfulGate, &logs)

	err := pipeline.landRebased(context.Background(), c, nil)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("landRebased() error = %v, want ErrConflict", err)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], c.Branch) {
		t.Errorf("warnings = %q, want one warning naming %q", logs, c.Branch)
	}
}

func TestLandRebasedRetryGateFailureDoesNotFastForward(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"merge", "--ff-only", "ifrit"}, Code: 1, Output: "diverging branches"},
		{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}},
	}}
	gates := 0
	pipeline := fastForwardPipeline(t, runner, func(context.Context, *Context) (int, error) {
		gates++
		if gates == 2 {
			return 3, nil
		}
		return 0, nil
	}, nil)

	err := pipeline.landRebased(context.Background(), c, nil)
	if !errors.Is(err, ErrGateFailed) {
		t.Fatalf("landRebased() error = %v, want ErrGateFailed", err)
	}
	if len(runner.calls) != 2 {
		t.Errorf("git calls = %q, want initial fast-forward and rebase only", runner.calls)
	}
}

func successfulGate(context.Context, *Context) (int, error) { return 0, nil }

func fastForwardContext() *Context {
	return &Context{Root: "/repo", Worktree: "/seat", Branch: "ifrit", Integration: "main"}
}

func fastForwardPipeline(t *testing.T, runner Runner, gate func(context.Context, *Context) (int, error), logs *[]string) *Pipeline {
	t.Helper()
	log := func(string, string) {}
	if logs != nil {
		log = func(level, message string) { *logs = append(*logs, level+": "+message) }
	}
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: runner, GateFunc: gate, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	return pipeline
}

func assertFastForwardArgs(t *testing.T, calls []fakeCall, want [][]string) {
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
