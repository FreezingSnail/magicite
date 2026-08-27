package land

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestRebaseSuccessUsesOnlyRequiredArguments(t *testing.T) {
	c := rebaseTestContext()
	fake := newFakeRunner(fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}})
	pipeline := rebaseTestPipeline(t, fake, nil)

	result, err := pipeline.rebase(context.Background(), c)
	if result != rebaseOK || err != nil {
		t.Fatalf("rebase() = (%v, %v), want (rebaseOK, nil)", result, err)
	}
	assertRebaseArgs(t, fake.Calls(), [][]string{{"-C", c.Worktree, "rebase", "main", "ifrit"}})
}

func TestRebaseConflictAbortsAndReturnsReportableResult(t *testing.T) {
	c := rebaseTestContext()
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}, Code: 1, Output: "CONFLICT (content)"},
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "--abort"}},
	)
	warnings := []string{}
	pipeline := rebaseTestPipeline(t, fake, &warnings)

	result, err := pipeline.rebase(context.Background(), c)
	if result != rebaseConflict || err != nil {
		t.Fatalf("rebase() = (%v, %v), want (rebaseConflict, nil)", result, err)
	}
	assertRebaseArgs(t, fake.Calls(), [][]string{
		{"-C", c.Worktree, "rebase", "main", "ifrit"},
		{"-C", c.Worktree, "rebase", "--abort"},
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %q, want none", warnings)
	}
}

func TestRebaseFailureAbortsAndLogsFailure(t *testing.T) {
	c := rebaseTestContext()
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}, Code: 2, Output: "bad ref"},
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "--abort"}},
	)
	var warnings []string
	pipeline := rebaseTestPipeline(t, fake, &warnings)

	result, err := pipeline.rebase(context.Background(), c)
	if result != rebaseFailed || err != nil {
		t.Fatalf("rebase() = (%v, %v), want (rebaseFailed, nil)", result, err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "exit 2") || !strings.Contains(warnings[0], "bad ref") {
		t.Errorf("warnings = %q, want failure exit and output", warnings)
	}
}

func TestRebaseAbortFailureIsLoggedWithoutChangingResult(t *testing.T) {
	c := rebaseTestContext()
	abortErr := errors.New("abort runner failed")
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}, Code: 2, Output: "bad ref"},
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "--abort"}, Code: 3, Err: abortErr},
	)
	var warnings []string
	pipeline := rebaseTestPipeline(t, fake, &warnings)

	result, err := pipeline.rebase(context.Background(), c)
	if result != rebaseFailed || err != nil {
		t.Fatalf("rebase() = (%v, %v), want (rebaseFailed, nil)", result, err)
	}
	if len(warnings) != 2 || !strings.Contains(warnings[0], "abort") || !strings.Contains(warnings[1], "bad ref") {
		t.Errorf("warnings = %q, want abort and rebase failure", warnings)
	}
}

func TestRebaseRunnerFailureReturnsErrorAfterAbort(t *testing.T) {
	c := rebaseTestContext()
	runErr := errors.New("runner failed")
	fake := newFakeRunner(
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "main", "ifrit"}, Code: -1, Err: runErr},
		fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", "--abort"}},
	)
	pipeline := rebaseTestPipeline(t, fake, nil)

	result, err := pipeline.rebase(context.Background(), c)
	if result != rebaseFailed || !errors.Is(err, runErr) {
		t.Fatalf("rebase() = (%v, %v), want rebaseFailed wrapping runner error", result, err)
	}
}

func TestLinearHistory(t *testing.T) {
	c := rebaseTestContext()
	for _, test := range []struct {
		name   string
		output string
		want   bool
	}{
		{name: "linear", output: "0\n", want: true},
		{name: "merge present", output: "1\n", want: false},
		{name: "noncanonical zero", output: "00\n", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeRunner(fakeReply{Prefix: []string{"-C", c.Worktree, "rev-list", "--count", "--merges", "main..ifrit"}, Output: test.output})
			pipeline := rebaseTestPipeline(t, fake, nil)
			got, err := pipeline.linear(context.Background(), c)
			if got != test.want || err != nil {
				t.Errorf("linear() = (%v, %v), want (%v, nil)", got, err, test.want)
			}
		})
	}
}

func TestLinearHistoryRejectsQueryAndMalformedCount(t *testing.T) {
	c := rebaseTestContext()
	queryErr := errors.New("query failed")
	pipeline := rebaseTestPipeline(t, newFakeRunner(fakeReply{
		Prefix: []string{"-C", c.Worktree, "rev-list"}, Code: 7, Output: "bad ref",
	}), nil)
	got, err := pipeline.linear(context.Background(), c)
	if got || err == nil || !strings.Contains(err.Error(), "bad ref") {
		t.Errorf("linear(query failure) = (%v, %v), want false with output error", got, err)
	}

	pipeline = rebaseTestPipeline(t, newFakeRunner(fakeReply{
		Prefix: []string{"-C", c.Worktree, "rev-list"}, Output: "not-a-count",
	}), nil)
	got, err = pipeline.linear(context.Background(), c)
	if got || !errors.Is(err, ErrNotLinear) {
		t.Errorf("linear(malformed count) = (%v, %v), want false and ErrNotLinear", got, err)
	}

	pipeline = rebaseTestPipeline(t, newFakeRunner(fakeReply{
		Prefix: []string{"-C", c.Worktree, "rev-list"}, Err: queryErr,
	}), nil)
	got, err = pipeline.linear(context.Background(), c)
	if got || !errors.Is(err, queryErr) {
		t.Errorf("linear(runner failure) = (%v, %v), want false wrapping query error", got, err)
	}
}

func rebaseTestContext() *Context {
	return &Context{Root: "/repo", Worktree: "/seat", Branch: "ifrit", Integration: "main"}
}

func rebaseTestPipeline(t *testing.T, runner Runner, warnings *[]string) *Pipeline {
	t.Helper()
	log := func(string, string) {}
	if warnings != nil {
		log = func(level, message string) { *warnings = append(*warnings, level+": "+message) }
	}
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: runner, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	return pipeline
}

func assertRebaseArgs(t *testing.T, calls []fakeCall, want [][]string) {
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
