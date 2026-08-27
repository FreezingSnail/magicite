package worktree

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestRootPathAndBranch(t *testing.T) {
	repo := initRepo(t)
	manager, err := New(Options{WorkspacePath: filepath.Join("harness", "workspaces")})
	if err != nil {
		t.Fatal(err)
	}

	root, err := manager.Root(repo)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(repo.root, "harness", "workspaces")
	if root != wantRoot || !filepath.IsAbs(root) {
		t.Errorf("Root() = %q, want absolute %q", root, wantRoot)
	}
	path, err := manager.Path(repo, "ifrit")
	if err != nil || path != filepath.Join(wantRoot, "ifrit") {
		t.Errorf("Path() = %q, %v; want %q, nil", path, err, filepath.Join(wantRoot, "ifrit"))
	}
	branch, err := manager.Branch(repo, "ifrit")
	if err != nil || branch != "ifrit" {
		t.Errorf("Branch() = %q, %v; want ifrit, nil", branch, err)
	}
}

func TestResolutionRefusesInvalidInputsWithoutGit(t *testing.T) {
	manager, err := New(Options{Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}
	repo := initRepo(t)
	badRepo := fakeRepo{name: "", root: repo.root, integration: "main"}
	if _, err := manager.Root(nil); !errors.Is(err, ErrUnresolvedRepo) {
		t.Errorf("Root(nil) error = %v, want ErrUnresolvedRepo", err)
	}
	if _, err := manager.Root(badRepo); !errors.Is(err, ErrUnresolvedRepo) {
		t.Errorf("Root(bad repo) error = %v, want ErrUnresolvedRepo", err)
	}
	for _, seat := range []string{"", " \t", ".", "..", "nested/seat", `nested\seat`} {
		if _, err := manager.Path(repo, seat); !errors.Is(err, ErrInvalidSeat) {
			t.Errorf("Path(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
		if _, err := manager.Branch(repo, seat); !errors.Is(err, ErrInvalidSeat) {
			t.Errorf("Branch(%q) error = %v, want ErrInvalidSeat", seat, err)
		}
	}
	if _, err := manager.Branch(repo, "main"); !errors.Is(err, ErrProtectedBranch) {
		t.Errorf("Branch(integration) error = %v, want ErrProtectedBranch", err)
	}
}

func TestRootRejectsEscapingWorkspace(t *testing.T) {
	repo := initRepo(t)
	manager := &Manager{workspacePath: filepath.Join("..", "outside")}
	if _, err := manager.Root(repo); !errors.Is(err, ErrEscapingPath) {
		t.Errorf("Root() error = %v, want ErrEscapingPath", err)
	}
}
