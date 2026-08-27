package land

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolve(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := fakeRepo{name: "fixture", root: root, integration: "main"}
	workspace := &fakeWorkspace{branch: "ifrit", path: worktree}
	pipeline, err := New(Options{Workspace: workspace, Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := pipeline.resolve(context.Background(), repo, "ifrit", true)
	if err != nil {
		t.Fatal(err)
	}
	want := &Context{Repo: repo, Root: root, Worktree: worktree, Branch: "ifrit", Integration: "main"}
	if !reflect.DeepEqual(resolved, want) {
		t.Errorf("resolve() = %#v, want %#v", resolved, want)
	}
	if got := workspace.Calls(); len(got) != 2 || got[0].Method != "Branch" || got[1].Method != "Path" {
		t.Errorf("workspace calls = %#v, want Branch then Path", got)
	}
}

func TestResolveRefusesInvalidInputs(t *testing.T) {
	root := t.TempDir()
	repo := fakeRepo{name: "fixture", root: root, integration: "main"}
	runner := newFakeRunner()
	workspace := &fakeWorkspace{branch: "ifrit", path: filepath.Join(root, "missing")}
	pipeline, err := New(Options{Workspace: workspace, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}

	for _, badRepo := range []Repo{
		nil,
		fakeRepo{root: root, integration: "main"},
		fakeRepo{name: "fixture", root: root},
		fakeRepo{name: "fixture", root: filepath.Join(root, "missing"), integration: "main"},
	} {
		if _, err := pipeline.resolve(context.Background(), badRepo, "ifrit", false); !errors.Is(err, ErrUnresolvedRepo) {
			t.Errorf("resolve(%#v) error = %v, want ErrUnresolvedRepo", badRepo, err)
		}
	}
	for _, seat := range []string{"", " \t", ".", ".."} {
		if _, err := pipeline.resolve(context.Background(), repo, seat, false); !errors.Is(err, ErrInvalidSeat) {
			t.Errorf("resolve(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
	}
	if _, err := pipeline.resolve(context.Background(), repo, "ifrit", true); !errors.Is(err, ErrMissingWorktree) {
		t.Errorf("resolve(missing worktree) error = %v, want ErrMissingWorktree", err)
	}
	if got := runner.Calls(); len(got) != 0 {
		t.Errorf("git calls = %#v, want none", got)
	}
}

func TestGitAndStatus(t *testing.T) {
	runner := newFakeRunner(
		fakeReply{Prefix: []string{"status"}, Code: 0, Output: "clean"},
		fakeReply{Prefix: []string{"-C", "worktree", "rebase"}, Code: 4, Output: "conflict"},
		fakeReply{Prefix: []string{"show"}, Code: -1, Err: errors.New("runner failed")},
	)
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	landing := &Context{Root: "root"}

	code, output, err := pipeline.git(context.Background(), landing, "root", "status")
	if code != 0 || output != "clean" || err != nil {
		t.Errorf("git(root) = %d, %q, %v; want 0, clean, nil", code, output, err)
	}
	code, output, err = pipeline.git(context.Background(), landing, "worktree", "rebase")
	if code != 4 || output != "conflict" || err != nil {
		t.Errorf("git(worktree) = %d, %q, %v; want 4, conflict, nil", code, output, err)
	}
	if got := pipeline.status(context.Background(), landing, "root", "show"); got != 1 {
		t.Errorf("status(seam error) = %d, want 1", got)
	}
	calls := runner.Calls()
	want := []fakeCall{
		{Dir: "root", Args: []string{"status"}},
		{Dir: "root", Args: []string{"-C", "worktree", "rebase"}},
		{Dir: "root", Args: []string{"show"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %#v, want %#v", calls, want)
	}
}
