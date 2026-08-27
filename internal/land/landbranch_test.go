package land

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/stamp"
)

func TestOutcomeString(t *testing.T) {
	for _, test := range []struct {
		outcome Outcome
		want    string
	}{
		{OutcomeFailed, "failed"},
		{OutcomeLanded, "landed"},
		{OutcomeConflict, "conflict"},
		{OutcomeGateFailed, "gate failed"},
		{Outcome(99), "unknown"},
	} {
		if got := test.outcome.String(); got != test.want {
			t.Errorf("Outcome(%d).String() = %q, want %q", test.outcome, got, test.want)
		}
	}
}

func TestLandBranchOrdersStepsAndSetsStampRepo(t *testing.T) {
	root, worktree := landBranchDirs(t)
	runner := &stampInspectRunner{orderedRunner: orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Code: 1},
		{Prefix: []string{"-C", worktree, "commit", "--amend", "--no-edit"}},
		{Prefix: []string{"rev-parse", "--verify", "ifrit"}},
		{Prefix: []string{"-C", worktree, "rebase", "main", "ifrit"}},
		{Prefix: []string{"-C", worktree, "rev-list", "--count", "--merges", "main..ifrit"}, Output: "0\n"},
		{Prefix: []string{"-C", worktree, "rev-list", "--reverse", "main..ifrit"}, Output: "tip\n"},
		{Prefix: []string{"-C", worktree, "rev-parse", "ifrit"}, Output: "tip\n"},
		{Prefix: []string{"-C", worktree, "log", "-1", "--format=%B", "ifrit"}, Output: "complete\n"},
		{Prefix: []string{"-C", worktree, "commit", "--amend", "-F"}},
		{Prefix: []string{"merge", "--ff-only", "ifrit"}},
	}}}
	pipeline := landBranchPipeline(t, root, worktree, runner, successfulGate)
	provided := &stamp.Stamp{Repo: "forged", Task: "magicite-ewp.10"}

	got, err := pipeline.LandBranch(context.Background(), fakeRepo{name: "resolved", root: root, integration: "main"}, "ifrit", provided)
	if got != OutcomeLanded || err != nil {
		t.Fatalf("LandBranch() = (%s, %v), want (landed, nil)", got, err)
	}
	if provided.Repo != "forged" {
		t.Errorf("provided stamp repo = %q, want unchanged forged", provided.Repo)
	}
	if !runner.stampedRepo {
		t.Errorf("stamped message = %q, want resolved repo", runner.message)
	}
	assertFastForwardArgs(t, runner.calls, [][]string{
		{"-C", worktree, "add", "-A"},
		{"merge-base", "--is-ancestor", "ifrit", "main"},
		{"-C", worktree, "commit", "--amend", "--no-edit"},
		{"rev-parse", "--verify", "ifrit"},
		{"-C", worktree, "rebase", "main", "ifrit"},
		{"-C", worktree, "rev-list", "--count", "--merges", "main..ifrit"},
		{"-C", worktree, "rev-list", "--reverse", "main..ifrit"},
		{"-C", worktree, "rev-parse", "ifrit"},
		{"-C", worktree, "log", "-1", "--format=%B", "ifrit"},
		{"-C", worktree, "commit", "--amend", "-F", runner.calls[9].Args[5]},
		{"merge", "--ff-only", "ifrit"},
	})
}

func TestLandBranchResolveRefusalMakesNoGitCall(t *testing.T) {
	runner := newFakeRunner()
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	got, err := pipeline.LandBranch(context.Background(), nil, "ifrit", nil)
	if got != OutcomeFailed || !errors.Is(err, ErrUnresolvedRepo) {
		t.Fatalf("LandBranch() = (%s, %v), want (failed, ErrUnresolvedRepo)", got, err)
	}
	if calls := runner.Calls(); len(calls) != 0 {
		t.Errorf("git calls = %#v, want none", calls)
	}
}

func TestLandBranchBranchMissingFails(t *testing.T) {
	root, worktree := landBranchDirs(t)
	runner := &orderedRunner{replies: []fakeReply{
		{Prefix: []string{"-C", worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Code: 1},
		{Prefix: []string{"-C", worktree, "commit", "--amend", "--no-edit"}},
		{Prefix: []string{"rev-parse", "--verify", "ifrit"}, Code: 128, Output: "unknown revision"},
	}}
	pipeline := landBranchPipeline(t, root, worktree, runner, successfulGate)

	got, err := pipeline.LandBranch(context.Background(), fakeRepo{name: "fixture", root: root, integration: "main"}, "ifrit", nil)
	if got != OutcomeFailed || !errors.Is(err, ErrBranchMissing) {
		t.Fatalf("LandBranch() = (%s, %v), want (failed, ErrBranchMissing)", got, err)
	}
	if len(runner.calls) != 4 {
		t.Errorf("git calls = %#v, want commit then branch verification only", runner.calls)
	}
}

func TestLandBranchRebaseConflictAndNonlinearRefuse(t *testing.T) {
	for _, test := range []struct {
		name    string
		replies func(string) []fakeReply
		wantErr error
	}{
		{
			name: "rebase conflict",
			replies: func(worktree string) []fakeReply {
				return append(landBranchBeforeRebase(worktree),
					fakeReply{Prefix: []string{"-C", worktree, "rebase", "main", "ifrit"}, Code: 1, Output: "CONFLICT"},
					fakeReply{Prefix: []string{"-C", worktree, "rebase", "--abort"}},
				)
			},
			wantErr: ErrConflict,
		},
		{
			name: "nonlinear",
			replies: func(worktree string) []fakeReply {
				return append(append(landBranchBeforeRebase(worktree),
					fakeReply{Prefix: []string{"-C", worktree, "rebase", "main", "ifrit"}},
				), fakeReply{Prefix: []string{"-C", worktree, "rev-list", "--count", "--merges", "main..ifrit"}, Output: "1\n"})
			},
			wantErr: ErrNotLinear,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, worktree := landBranchDirs(t)
			runner := &orderedRunner{replies: test.replies(worktree)}
			pipeline := landBranchPipeline(t, root, worktree, runner, successfulGate)

			got, err := pipeline.LandBranch(context.Background(), fakeRepo{name: "fixture", root: root, integration: "main"}, "ifrit", nil)
			if got != OutcomeConflict || !errors.Is(err, test.wantErr) || err == nil {
				t.Fatalf("LandBranch() = (%s, %v), want (conflict, %v)", got, err, test.wantErr)
			}
			if test.wantErr == ErrNotLinear && !errors.Is(err, ErrConflict) {
				t.Errorf("nonlinear error = %v, want ErrConflict too", err)
			}
		})
	}
}

func TestLandBranchMapsGateFailure(t *testing.T) {
	root, worktree := landBranchDirs(t)
	runner := &orderedRunner{replies: append(append(landBranchBeforeRebase(worktree),
		fakeReply{Prefix: []string{"-C", worktree, "rebase", "main", "ifrit"}},
	), fakeReply{Prefix: []string{"-C", worktree, "rev-list", "--count", "--merges", "main..ifrit"}, Output: "0\n"})}
	pipeline := landBranchPipeline(t, root, worktree, runner, func(context.Context, *Context) (int, error) { return 1, nil })

	got, err := pipeline.LandBranch(context.Background(), fakeRepo{name: "fixture", root: root, integration: "main"}, "ifrit", nil)
	if got != OutcomeGateFailed || !errors.Is(err, ErrGateFailed) || err == nil {
		t.Fatalf("LandBranch() = (%s, %v), want (gate failed, ErrGateFailed)", got, err)
	}
	if len(runner.calls) != 6 {
		t.Errorf("git calls = %#v, want no fast-forward after gate failure", runner.calls)
	}
}

func landBranchDirs(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, worktree
}

func landBranchPipeline(t *testing.T, root, worktree string, runner Runner, gate func(context.Context, *Context) (int, error)) *Pipeline {
	t.Helper()
	pipeline, err := New(Options{
		Workspace: &fakeWorkspace{branch: "ifrit", path: worktree},
		Runner:    runner,
		GateFunc:  gate,
	})
	if err != nil {
		t.Fatal(err)
	}
	return pipeline
}

func landBranchBeforeRebase(worktree string) []fakeReply {
	return []fakeReply{
		{Prefix: []string{"-C", worktree, "add", "-A"}},
		{Prefix: []string{"merge-base", "--is-ancestor", "ifrit", "main"}, Code: 1},
		{Prefix: []string{"-C", worktree, "commit", "--amend", "--no-edit"}},
		{Prefix: []string{"rev-parse", "--verify", "ifrit"}},
	}
}

type stampInspectRunner struct {
	orderedRunner
	message     string
	stampedRepo bool
}

func (r *stampInspectRunner) Git(ctx context.Context, dir string, args ...string) (int, string, error) {
	if len(args) >= 6 && args[0] == "-C" && args[2] == "commit" && args[3] == "--amend" && args[4] == "-F" {
		message, err := os.ReadFile(args[5])
		if err != nil {
			return -1, "", err
		}
		r.message = string(message)
		r.stampedRepo = strings.Contains(r.message, "Magicite-Repo: resolved")
	}
	return r.orderedRunner.Git(ctx, dir, args...)
}
