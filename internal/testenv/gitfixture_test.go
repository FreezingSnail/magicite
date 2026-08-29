package testenv

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Fixture repositories skip the git template, so no sample hooks are copied. The
// suite created 28 repositories in a 20.5 s window and 182 of its git writes were
// hooks no test runs.
func TestRepoSkipsHookTemplate(t *testing.T) {
	repo := NewRepo(t, New(t), "template")
	entries, err := os.ReadDir(filepath.Join(repo.Root, ".git", "hooks"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Errorf("fixture repository copied hook %q, want an empty hooks directory", entry.Name())
	}
}

func TestRepoDeterministicHistory(t *testing.T) {
	build := func(name string) (string, string) {
		t.Helper()
		repo := NewRepo(t, New(t), name)
		initial := repo.Head("main")
		return initial, repo.Commit("add files", map[string]string{
			"z.txt":        "z\n",
			"nested/a.txt": "a\n",
		})
	}

	initialOne, headOne := build("one")
	initialTwo, headTwo := build("two")
	if initialOne != initialTwo {
		t.Errorf("initial SHA = %s and %s, want identical", initialOne, initialTwo)
	}
	if headOne != headTwo {
		t.Errorf("commit SHA = %s and %s, want identical", headOne, headTwo)
	}
}

func TestRepoBranchCheckoutWorktreeAndHistory(t *testing.T) {
	env := New(t)
	repo := NewRepo(t, env, "source")
	initial := repo.Head("main")
	repo.Branch("feature", "main")
	repo.Checkout("feature")
	feature := repo.Commit("feature change", map[string]string{"feature.txt": "feature\n"})
	repo.Checkout("main")
	main := repo.Commit("main change", map[string]string{"main.txt": "main\n"})
	repo.Branch("worker", "feature")
	worktree := repo.Worktree("shiva", "worker")

	if !fixturePath(env.Root, worktree) {
		t.Errorf("worktree %q outside environment root %q", worktree, env.Root)
	}
	if info, err := os.Stat(filepath.Join(worktree, ".git")); err != nil || info.IsDir() {
		t.Errorf("worktree .git = %v, want gitdir file", err)
	}
	if got, want := repo.Head("feature"), feature; got != want {
		t.Errorf("feature head = %s, want %s", got, want)
	}
	if got, want := repo.Head("main"), main; got != want {
		t.Errorf("main head = %s, want %s", got, want)
	}
	if got, want := repo.Parents(feature), []string{initial}; !reflect.DeepEqual(got, want) {
		t.Errorf("feature parents = %#v, want %#v", got, want)
	}
	if got, want := repo.Log("main"), []string{"main change", "initial"}; !reflect.DeepEqual(got, want) {
		t.Errorf("main log = %#v, want %#v", got, want)
	}
	if !repo.Linear("main") {
		t.Error("main history = non-linear, want linear")
	}
	if !repo.Exists("worker") || repo.Exists("missing") {
		t.Errorf("Exists(worker, missing) = (%t, %t), want (true, false)", repo.Exists("worker"), repo.Exists("missing"))
	}
}

func TestRepoTrailersPreserveDuplicateKeysAndMergesAreNonlinear(t *testing.T) {
	repo := NewRepo(t, New(t), "source")
	repo.Branch("feature", "main")
	repo.Checkout("feature")
	repo.Commit("feature\n\nSigned-off-by: first\nSigned-off-by: second\nReview: approved", map[string]string{"feature.txt": "feature\n"})
	trailers := repo.Trailers("HEAD")
	wantTrailers := []Trailer{
		{Key: "Signed-off-by", Value: "first"},
		{Key: "Signed-off-by", Value: "second"},
		{Key: "Review", Value: "approved"},
	}
	if !reflect.DeepEqual(trailers, wantTrailers) {
		t.Errorf("trailers = %#v, want %#v", trailers, wantTrailers)
	}

	repo.Checkout("main")
	repo.Commit("main", map[string]string{"main.txt": "main\n"})
	repo.git("merge", "--no-ff", "--no-edit", "feature")
	if repo.Linear("HEAD") {
		t.Error("merged history = linear, want non-linear")
	}
	if got := len(repo.Parents(repo.Head("HEAD"))); got != 2 {
		t.Errorf("merge parents = %d, want 2", got)
	}
}
