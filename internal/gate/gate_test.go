package gate

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func testRepo(t *testing.T, name string) repo.Repo {
	t.Helper()
	r, ok := repo.Make("/work/"+name, name, name, "main")
	if !ok {
		t.Fatal("repo.Make() failed")
	}
	return r
}

func testGate(t *testing.T) *Gate {
	t.Helper()
	g, err := New(Deps{Beads: &fakeBeads{}, Git: &fakeGit{}, Repos: &fakeRepos{records: map[string]repo.Repo{}}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return g
}

func TestNewValidatesPortsInOrder(t *testing.T) {
	for _, test := range []struct {
		name string
		deps Deps
		want string
	}{
		{"beads", Deps{}, "Beads"},
		{"git", Deps{Beads: &fakeBeads{}}, "Git"},
		{"repos", Deps{Beads: &fakeBeads{}, Git: &fakeGit{}}, "Repos"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.deps)
			var missing *MissingDependencyError
			if !errors.As(err, &missing) || missing.Dependency != test.want {
				t.Fatalf("New() error = %v, want missing %s", err, test.want)
			}
		})
	}
}

func TestNewRejectsTypedNilPort(t *testing.T) {
	var beads *fakeBeads
	_, err := New(Deps{Beads: beads, Git: &fakeGit{}, Repos: &fakeRepos{}})
	var missing *MissingDependencyError
	if !errors.As(err, &missing) || missing.Dependency != "Beads" {
		t.Fatalf("New() error = %v, want missing Beads", err)
	}
}

func TestNewNormalizesConfig(t *testing.T) {
	deps := Deps{Beads: &fakeBeads{}, Git: &fakeGit{}, Repos: &fakeRepos{}}
	g, err := New(deps)
	if err != nil || g.MaxRetries() != defaultMaxRetries || g.Enabled() {
		t.Fatalf("New() = (%v, %v), want disabled retries %d", g, err, defaultMaxRetries)
	}
	for _, config := range []Config{{Enabled: true, Agent: "agent"}, {Enabled: true, Model: "model"}} {
		_, err := New(Deps{Beads: &fakeBeads{}, Git: &fakeGit{}, Repos: &fakeRepos{}, Config: config})
		var invalid *InvalidConfigError
		if !errors.As(err, &invalid) {
			t.Fatalf("New(%+v) error = %v, want InvalidConfigError", config, err)
		}
	}
}

func TestStateIsRepositoryScoped(t *testing.T) {
	g := testGate(t)
	left, right := testRepo(t, "left"), testRepo(t, "right")
	leftKey, ok := g.key(left, "epic")
	if !ok {
		t.Fatal("left key rejected")
	}
	rightKey, ok := g.key(right, "epic")
	if !ok {
		t.Fatal("right key rejected")
	}
	g.noteAttempt(leftKey)
	g.track("left-handle", leftKey)
	g.recordStart(leftKey, "left-sha")
	if got := g.attempts(rightKey); got != 0 {
		t.Fatalf("right attempts = %d, want 0", got)
	}
	if g.inFlight(right) {
		t.Fatal("right unexpectedly in flight")
	}
	if _, ok := g.start(rightKey); ok {
		t.Fatal("right unexpectedly started")
	}
}

func TestKeyRejectsIncompleteInput(t *testing.T) {
	g := testGate(t)
	r := testRepo(t, "repo")
	for _, test := range []struct {
		name string
		repo repo.Repo
		epic string
	}{
		{"zero repo", repo.Repo{}, "epic"},
		{"empty name", repo.Repo{Root: "/work/repo/"}, "epic"},
		{"empty epic", r, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := g.key(test.repo, test.epic); ok {
				t.Fatal("key() accepted incomplete input")
			}
		})
	}
}

func TestStateLifecycle(t *testing.T) {
	g := testGate(t)
	r := testRepo(t, "repo")
	k, _ := g.key(r, "epic")
	g.noteAttempt(k)
	g.noteAttempt(k)
	if got := g.attempts(k); got != 2 {
		t.Fatalf("attempts() = %d, want 2", got)
	}
	if g.exhaust(k) || !g.exhaust(k) {
		t.Fatal("exhaust() did not report first then repeated record")
	}
	if !g.recordStart(k, "first") || g.recordStart(k, "second") {
		t.Fatal("recordStart() did not retain first SHA")
	}
	if got, ok := g.start(k); !ok || got != "first" {
		t.Fatalf("start() = (%q, %v), want (first, true)", got, ok)
	}
	g.track("handle", k)
	if !g.inFlight(r) {
		t.Fatal("inFlight() = false, want true")
	}
	if got, ok := g.drop("handle"); !ok || got != k {
		t.Fatalf("drop() = (%#v, %v), want (%#v, true)", got, ok, k)
	}
	g.clear(k)
	if g.attempts(k) != 0 {
		t.Fatal("clear() retained attempts")
	}
}

func TestResolveAndReset(t *testing.T) {
	r := testRepo(t, "repo")
	other := testRepo(t, "other")
	repos := &fakeRepos{records: map[string]repo.Repo{r.Name: r, other.Name: other}}
	g, err := New(Deps{Beads: &fakeBeads{}, Git: &fakeGit{}, Repos: repos})
	if err != nil {
		t.Fatal(err)
	}
	k, _ := g.key(r, "epic")
	otherKey, _ := g.key(other, "epic")
	g.track("handle", k)
	g.noteAttempt(k)
	g.exhaust(k)
	g.recordStart(k, "sha")
	g.noteAttempt(otherKey)
	gotRepo, epic, ok := g.resolve("handle")
	if !ok || gotRepo != r || epic != "epic" {
		t.Fatalf("resolve() = (%#v, %q, %v)", gotRepo, epic, ok)
	}
	repos.mu.Lock()
	delete(repos.records, r.Name)
	repos.mu.Unlock()
	if _, _, ok := g.resolve("handle"); ok {
		t.Fatal("resolve() found vanished repository")
	}
	g.Reset(r)
	if g.attempts(k) != 0 || g.attempts(otherKey) != 1 || g.inFlight(r) {
		t.Fatal("Reset(repo) retained or cleared wrong attempt state")
	}
	if _, ok := g.start(k); ok || g.exhaust(k) {
		t.Fatal("Reset(repo) retained start or exhaustion state")
	}
	g.Reset(repo.Repo{})
	if g.attempts(otherKey) != 0 {
		t.Fatal("Reset(nil) retained state")
	}
}

func TestFakesRecordAndScriptPorts(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{show: func(_ context.Context, got repo.Repo, id string) (bd.Bead, error) {
		if got != r || id != "bead" {
			t.Fatalf("Show() = (%#v, %q)", got, id)
		}
		return bd.Bead{ID: id}, nil
	}}
	if bead, err := beads.Show(context.Background(), r, "bead"); err != nil || bead.ID != "bead" {
		t.Fatalf("Show() = (%#v, %v)", bead, err)
	}
	if calls := beads.Calls(); len(calls) != 1 || calls[0].method != "Show" {
		t.Fatalf("bead calls = %#v", calls)
	}
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		return 7, "output", nil
	}}
	if status, output, err := git.Output(context.Background(), r, "rev-parse", "HEAD"); status != 7 || output != "output" || err != nil {
		t.Fatalf("Output() = (%d, %q, %v)", status, output, err)
	}
	if calls := git.Calls(); len(calls) != 1 || len(calls[0]) != 2 || calls[0][0] != "rev-parse" {
		t.Fatalf("git calls = %#v", calls)
	}
}

func TestStateConcurrentAccess(t *testing.T) {
	g := testGate(t)
	r := testRepo(t, "repo")
	k, _ := g.key(r, "epic")
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 100; j++ {
				g.noteAttempt(k)
				g.exhaust(k)
				g.recordStart(k, "sha")
				g.track("handle", k)
				g.inFlight(r)
				g.start(k)
				g.drop("handle")
			}
		}()
	}
	group.Wait()
}
