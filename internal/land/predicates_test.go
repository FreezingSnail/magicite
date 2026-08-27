package land

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestLanded(t *testing.T) {
	root := t.TempDir()
	repo := fakeRepo{name: "fixture", root: root, integration: "main"}
	tests := []struct {
		name     string
		reply    fakeReply
		want     bool
		wantErr  bool
		worktree string
	}{
		{name: "ancestor", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}}, want: true, worktree: "/removed"},
		{name: "not ancestor", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Code: 1}, worktree: "/removed"},
		{name: "query failure", reply: fakeReply{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Code: 2, Output: "bad ref"}, wantErr: true, worktree: "/removed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newFakeRunner(test.reply)
			workspace := &fakeWorkspace{branch: "ifrit", path: test.worktree}
			pipeline, err := New(Options{Workspace: workspace, Runner: runner})
			if err != nil {
				t.Fatal(err)
			}
			got, err := pipeline.Landed(context.Background(), repo, "ifrit")
			if got != test.want || (err != nil) != test.wantErr {
				t.Errorf("Landed() = (%v, %v), want (%v, error=%v)", got, err, test.want, test.wantErr)
			}
			if calls := runner.Calls(); len(calls) != 1 || calls[0].Dir != root || !slices.Equal(calls[0].Args, []string{"merge-base", "--is-ancestor", "ifrit", "main"}) {
				t.Errorf("git calls = %#v, want root ancestry query", calls)
			}
			if calls := workspace.Calls(); len(calls) != 2 || workspace.path == "" {
				t.Errorf("workspace calls = %#v, want branch and path resolution", calls)
			}
		})
	}
}

func TestTaskLandedMatchesExactStamp(t *testing.T) {
	root := t.TempDir()
	repo := fakeRepo{name: "fixture", root: root, integration: "main"}
	for _, test := range []struct {
		name   string
		output string
		task   string
		want   bool
	}{
		{name: "exact", output: "\n magicite-cld \nmagicite-ewp.8\n", task: "magicite-ewp.8", want: true},
		{name: "prefix is not exact", output: "magicite-cld\n", task: "magicite-cl", want: false},
		{name: "missing", output: "magicite-other\n", task: "magicite-ewp.8", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := newFakeRunner(fakeReply{Prefix: []string{"log", "main", "--format=%(trailers:key=Magicite-Task,valueonly)"}, Output: test.output})
			pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: runner})
			if err != nil {
				t.Fatal(err)
			}
			got, err := pipeline.TaskLanded(context.Background(), repo, test.task)
			if got != test.want || err != nil {
				t.Errorf("TaskLanded() = (%v, %v), want (%v, nil)", got, err, test.want)
			}
			calls := runner.Calls()
			if len(calls) != 1 || calls[0].Dir != root || !slices.Equal(calls[0].Args, []string{"log", "main", "--format=%(trailers:key=Magicite-Task,valueonly)"}) {
				t.Errorf("git calls = %#v, want root trailer query", calls)
			}
		})
	}
}

func TestTaskLandedRefusesInvalidInputAndQueryFailure(t *testing.T) {
	root := t.TempDir()
	repo := fakeRepo{name: "fixture", root: root, integration: "main"}
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := pipeline.TaskLanded(context.Background(), nil, "task"); got || !errors.Is(err, ErrUnresolvedRepo) {
		t.Errorf("TaskLanded(nil repo) = (%v, %v), want false and ErrUnresolvedRepo", got, err)
	}
	if got, err := pipeline.TaskLanded(context.Background(), repo, " "); got || !errors.Is(err, ErrTaskUnstamped) {
		t.Errorf("TaskLanded(empty task) = (%v, %v), want false and ErrTaskUnstamped", got, err)
	}

	queryErr := errors.New("runner failed")
	pipeline, err = New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner(fakeReply{
		Prefix: []string{"log", "main"}, Err: queryErr,
	})})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := pipeline.TaskLanded(context.Background(), repo, "task"); got || !errors.Is(err, queryErr) {
		t.Errorf("TaskLanded(query failure) = (%v, %v), want false and query error", got, err)
	}
}

func TestAssertTaskLanded(t *testing.T) {
	root := t.TempDir()
	repo := fakeRepo{name: "fixture", root: root, integration: "main"}
	makePipeline := func(output string) *Pipeline {
		pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner(fakeReply{
			Prefix: []string{"log", "main"}, Output: output,
		})})
		if err != nil {
			t.Fatal(err)
		}
		return pipeline
	}

	if err := makePipeline("task\n").AssertTaskLanded(context.Background(), repo, "task"); err != nil {
		t.Fatalf("AssertTaskLanded(landed) = %v, want nil", err)
	}
	err := makePipeline("other\n").AssertTaskLanded(context.Background(), repo, "task")
	if !errors.Is(err, ErrTaskUnstamped) || !strings.Contains(err.Error(), "task") || !strings.Contains(err.Error(), "fixture") {
		t.Errorf("AssertTaskLanded(unlanded) = %v, want named ErrTaskUnstamped", err)
	}

	queryErr := errors.New("runner failed")
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner(fakeReply{Prefix: []string{"log", "main"}, Err: queryErr})})
	if err != nil {
		t.Fatal(err)
	}
	if err := pipeline.AssertTaskLanded(context.Background(), repo, "task"); !errors.Is(err, queryErr) || errors.Is(err, ErrTaskUnstamped) {
		t.Errorf("AssertTaskLanded(query failure) = %v, want underlying query error only", err)
	}
}
