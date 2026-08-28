package parity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
	magicexec "github.com/connorfranc/magicite/internal/exec"
	"github.com/connorfranc/magicite/internal/land"
	"github.com/connorfranc/magicite/internal/repo"
	"github.com/connorfranc/magicite/internal/stamp"
	"github.com/connorfranc/magicite/internal/testenv"
)

func TestLandParity(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		env, repository, worktree, pipeline := landFixture(t)
		before := repository.Head("main")
		writeFile(t, filepath.Join(worktree, "feature.txt"), "feature\n")
		resetTrace(t, env)

		outcome, err := pipeline.LandBranch(context.Background(), repositoryRef{repository}, "ifrit", parityStamp("task-1"))
		if outcome != land.OutcomeLanded || err != nil {
			t.Fatalf("LandBranch() = (%s, %v)", outcome, err)
		}
		if parents := repository.Parents("main"); len(parents) != 1 || parents[0] != before {
			t.Fatalf("main parents = %v, want fast-forward from %s", parents, before)
		}
		if !repository.Linear("main") {
			t.Fatal("main history contains a merge")
		}
		assertTrailers(t, repository.Trailers("main"), parityStamp("task-1").Trailers())
		AssertTrace(t, "land-clean", readTrace(t, env))
	})

	t.Run("rebase conflict", func(t *testing.T) {
		env, repository, worktree, pipeline := landFixture(t)
		writeFile(t, filepath.Join(worktree, "shared.txt"), "seat\n")
		runGit(t, env, worktree, "add", "shared.txt")
		runGit(t, env, worktree, "commit", "-m", "seat change")
		repository.Commit("main change", map[string]string{"shared.txt": "main\n"})
		before := repository.Head("main")
		resetTrace(t, env)

		outcome, err := pipeline.LandBranch(context.Background(), repositoryRef{repository}, "ifrit", parityStamp("task-1"))
		if outcome != land.OutcomeConflict || !errors.Is(err, land.ErrConflict) {
			t.Fatalf("LandBranch() = (%s, %v), want conflict", outcome, err)
		}
		if repository.Head("main") != before {
			t.Fatal("conflict advanced main")
		}
		entries := readTrace(t, env)
		for _, entry := range entries {
			if slices.Contains(entry.Argv, "merge") || slices.Contains(entry.Argv, "close") {
				t.Fatalf("conflict trace contains forbidden argv %q", entry.Argv)
			}
		}
		AssertTrace(t, "land-rebase-conflict", entries)
	})

	t.Run("idempotent stamp", func(t *testing.T) {
		env, repository, worktree, pipeline := landFixture(t)
		writeFile(t, filepath.Join(worktree, "feature.txt"), "feature\n")
		resetTrace(t, env)
		for range 2 {
			outcome, err := pipeline.LandBranch(context.Background(), repositoryRef{repository}, "ifrit", parityStamp("task-1"))
			if outcome != land.OutcomeLanded || err != nil {
				t.Fatalf("LandBranch() = (%s, %v)", outcome, err)
			}
		}
		assertTrailers(t, repository.Trailers("main"), parityStamp("task-1").Trailers())
		AssertTrace(t, "stamp-idempotent", readTrace(t, env))
	})
}

func TestCloseAfterLandParity(t *testing.T) {
	env, repository, worktree, pipeline := landFixture(t)
	beads := &parityBeads{env: env, beads: map[string]bd.Bead{"task-1": {ID: "task-1", Status: "open"}}}
	writeFile(t, filepath.Join(worktree, "feature.txt"), "feature\n")
	resetTrace(t, env)

	if err := pipeline.AssertTaskLanded(context.Background(), repositoryRef{repository}, "task-1"); !errors.Is(err, land.ErrTaskUnstamped) {
		t.Fatalf("AssertTaskLanded() = %v, want unstamped", err)
	}
	if beads.closed("task-1") {
		t.Fatal("unstamped task was closed")
	}
	if outcome, err := pipeline.LandBranch(context.Background(), repositoryRef{repository}, "ifrit", parityStamp("task-1")); outcome != land.OutcomeLanded || err != nil {
		t.Fatalf("LandBranch() = (%s, %v)", outcome, err)
	}
	if err := pipeline.AssertTaskLanded(context.Background(), repositoryRef{repository}, "task-1"); err != nil {
		t.Fatal(err)
	}
	record, ok := repo.Make(repository.Root, "repo", "repo", "main")
	if !ok {
		t.Fatal("make fixture repository record")
	}
	if err := beads.Close(context.Background(), record, "task-1", "landed"); err != nil {
		t.Fatal(err)
	}
	if !beads.closed("task-1") {
		t.Fatal("landed task was not closed")
	}
	AssertTrace(t, "close-after-land", readTrace(t, env))
}

type repositoryRef struct{ *testenv.Repo }

func (r repositoryRef) Name() string        { return r.Repo.Name }
func (r repositoryRef) Root() string        { return r.Repo.Root }
func (r repositoryRef) Integration() string { return "main" }

type fixtureWorkspace struct{ path string }

func (w fixtureWorkspace) Branch(land.Repo, string) (string, error) { return "ifrit", nil }
func (w fixtureWorkspace) Path(land.Repo, string) (string, error)   { return w.path, nil }

type tracedGit struct {
	t   *testing.T
	env *testenv.Env
}

func (g tracedGit) Git(ctx context.Context, dir string, args ...string) (int, string, error) {
	g.t.Helper()
	recorded := append([]string{"git"}, args...)
	if err := testenv.Record(g.env.TracePath, recorded, dir); err != nil {
		g.t.Fatal(err)
	}
	stdout, stderr, code, err := magicexec.RunEnv(ctx, dir, g.env.Env(), "git", args...)
	if code >= 0 {
		return code, string(stdout) + string(stderr), nil
	}
	return code, string(stdout) + string(stderr), err
}

func landFixture(t *testing.T) (*testenv.Env, *testenv.Repo, string, *land.Pipeline) {
	t.Helper()
	env := testenv.New(t)
	repository := testenv.NewRepo(t, env, "repo")
	repository.Branch("ifrit", "main")
	worktree := repository.Worktree("ifrit", "ifrit")
	pipeline, err := land.New(land.Options{Workspace: fixtureWorkspace{path: worktree}, Runner: tracedGit{t: t, env: env}, GateFunc: func(context.Context, *land.Context) (int, error) { return 0, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return env, repository, worktree, pipeline
}

func parityStamp(task string) *stamp.Stamp {
	return &stamp.Stamp{Model: "model", Backend: "backend", Difficulty: "high", Effort: "high", Agent: "agent", Repo: "repo", Seat: "ifrit", Task: task, Harness: "maduin 0.3.0", HarnessRev: "revision"}
}

func assertTrailers(t *testing.T, got []testenv.Trailer, want []stamp.Trailer) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("trailers = %#v, want %#v", got, want)
	}
	for index, trailer := range want {
		if got[index].Key != trailer.Key || got[index].Value != trailer.Value {
			t.Fatalf("trailer[%d] = %#v, want %#v", index, got[index], trailer)
		}
	}
}

func runGit(t *testing.T, env *testenv.Env, dir string, args ...string) {
	t.Helper()
	stdout, stderr, code, err := magicexec.RunEnv(context.Background(), dir, env.Env(), "git", args...)
	if err != nil || code != 0 {
		t.Fatalf("git %q = (%d, %v): %s%s", args, code, err, stdout, stderr)
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resetTrace(t *testing.T, env *testenv.Env) {
	t.Helper()
	if err := testenv.Reset(env.TracePath); err != nil {
		t.Fatal(err)
	}
}

func readTrace(t *testing.T, env *testenv.Env) []testenv.Entry {
	t.Helper()
	entries, err := testenv.Read(env.TracePath)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

type parityBeads struct {
	env   *testenv.Env
	beads map[string]bd.Bead
}

func (b *parityBeads) record(dir string, argv ...string) error {
	return testenv.Record(b.env.TracePath, append([]string{"bd"}, argv...), dir)
}

func (b *parityBeads) closed(id string) bool { return b.beads[id].Status == "closed" }
