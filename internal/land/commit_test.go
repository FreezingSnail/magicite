package land

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCommitCreatesFreshCommitAfterLanding(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", c.Worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", c.Branch, c.Integration}},
		{Prefix: []string{"-C", c.Worktree, "commit", "-m", "task complete (shiva)"}},
	}}
	pipeline := fastForwardPipeline(t, runner, nil, nil)

	if err := pipeline.commit(context.Background(), c, "shiva"); err != nil {
		t.Fatal(err)
	}
	want := []fakeCall{
		{Dir: c.Root, Args: []string{"-C", c.Worktree, "add", "-A"}},
		{Dir: c.Root, Args: []string{"merge-base", "--is-ancestor", c.Branch, c.Integration}},
		{Dir: c.Root, Args: []string{"-C", c.Worktree, "commit", "-m", "task complete (shiva)"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Errorf("git calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCommitAmendsUnlandedBranch(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", c.Worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", c.Branch, c.Integration}, Code: 1},
		{Prefix: []string{"-C", c.Worktree, "commit", "--amend", "--no-edit"}},
	}}
	pipeline := fastForwardPipeline(t, runner, nil, nil)

	if err := pipeline.commit(context.Background(), c, "shiva"); err != nil {
		t.Fatal(err)
	}
	if got := runner.calls[2].Args; !reflect.DeepEqual(got, []string{"-C", c.Worktree, "commit", "--amend", "--no-edit"}) {
		t.Errorf("commit argv = %q, want amend argv", got)
	}
}

func TestCommitAcceptsNothingToCommit(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", c.Worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", c.Branch, c.Integration}},
		{Prefix: []string{"-C", c.Worktree, "commit"}, Code: 1, Output: "nothing to commit, working tree clean"},
	}}
	var logs []string
	pipeline := fastForwardPipeline(t, runner, nil, &logs)

	if err := pipeline.commit(context.Background(), c, "shiva"); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 0 {
		t.Errorf("warnings = %q, want none", logs)
	}
}

func TestCommitWarnsOnceForCommitFailure(t *testing.T) {
	c := fastForwardContext()
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", c.Worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", c.Branch, c.Integration}},
		{Prefix: []string{"-C", c.Worktree, "commit"}, Code: 7, Output: "hook rejected"},
	}}
	var logs []string
	pipeline := fastForwardPipeline(t, runner, nil, &logs)

	err := pipeline.commit(context.Background(), c, "shiva")
	if err == nil || !strings.Contains(err.Error(), "exit 7") || !strings.Contains(err.Error(), "hook rejected") {
		t.Fatalf("commit() error = %v, want exit and output", err)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "hook rejected") {
		t.Errorf("warnings = %q, want one commit warning", logs)
	}
}

func TestCommitWrapsGitSeamError(t *testing.T) {
	c := fastForwardContext()
	cause := errors.New("runner unavailable")
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", c.Worktree, "add", "-A"}, Err: cause},
	}}
	pipeline := fastForwardPipeline(t, runner, nil, nil)

	err := pipeline.commit(context.Background(), c, "shiva")
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "stage worktree") {
		t.Fatalf("commit() error = %v, want wrapped seam cause", err)
	}
	if len(runner.calls) != 1 {
		t.Errorf("git calls = %d, want one", len(runner.calls))
	}
}
