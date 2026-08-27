package land

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testCommitDate = "2000-01-01T00:00:00+0000"

type testRepo struct {
	name        string
	root        string
	integration string
	seats       map[string]string
}

func newTestRepo(t *testing.T, name string) *testRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	r := &testRepo{
		name:        name,
		root:        filepath.Join(t.TempDir(), name),
		integration: "main",
		seats:       make(map[string]string),
	}
	if err := os.Mkdir(r.root, 0o755); err != nil {
		t.Fatal(err)
	}
	r.git(t, r.root, "init", "--initial-branch=main")
	r.git(t, r.root, "config", "user.name", "Magicite Test")
	r.git(t, r.root, "config", "user.email", "test@example.invalid")
	r.git(t, r.root, "config", "commit.gpgsign", "false")
	r.git(t, r.root, "config", "rebase.rebaseMerges", "true")
	r.write(t, "README", "fixture\n")
	r.commitAll(t, "initial")
	return r
}

func (r *testRepo) Name() string        { return r.name }
func (r *testRepo) Root() string        { return r.root }
func (r *testRepo) Integration() string { return r.integration }

func (r *testRepo) seat(t *testing.T, seat string) string {
	t.Helper()
	if _, ok := r.seats[seat]; ok {
		t.Fatalf("seat %q already exists", seat)
	}
	path := filepath.Join(filepath.Dir(r.root), "seat-"+seat)
	r.git(t, r.root, "worktree", "add", "-b", seat, path, r.integration)
	r.seats[seat] = path
	return path
}

func (r *testRepo) write(t *testing.T, rel, content string) {
	t.Helper()
	r.writeAt(t, r.root, rel, content)
}

func (r *testRepo) writeAt(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *testRepo) commitAll(t *testing.T, message string) {
	t.Helper()
	r.commitAllAt(t, r.root, message)
}

func (r *testRepo) commitAllAt(t *testing.T, dir, message string, config ...string) {
	t.Helper()
	r.git(t, dir, "add", "-A")
	args := append(append([]string(nil), config...), "commit", "-m", message)
	r.git(t, dir, args...)
}

func (r *testRepo) log(t *testing.T, format string, revs ...string) []string {
	t.Helper()
	args := append([]string{"log", "--format=" + format}, revs...)
	output := r.output(t, r.root, args...)
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

func (r *testRepo) workspace() Workspace { return testWorkspace{repo: r} }

func (r *testRepo) git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if output, code, err := testGit(context.Background(), dir, args...); err != nil || code != 0 {
		t.Fatalf("git %q: exit %d, error %v\n%s", args, code, err, output)
	}
}

func (r *testRepo) output(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, code, err := testGit(context.Background(), dir, args...)
	if err != nil || code != 0 {
		t.Fatalf("git %q: exit %d, error %v\n%s", args, code, err, output)
	}
	return output
}

type testWorkspace struct{ repo *testRepo }

func (w testWorkspace) Branch(repo Repo, seat string) (string, error) {
	if repo != w.repo {
		return "", fmt.Errorf("unknown repository")
	}
	if _, ok := w.repo.seats[seat]; !ok {
		return "", fmt.Errorf("unknown seat %q", seat)
	}
	return seat, nil
}

func (w testWorkspace) Path(repo Repo, seat string) (string, error) {
	if repo != w.repo {
		return "", fmt.Errorf("unknown repository")
	}
	path, ok := w.repo.seats[seat]
	if !ok {
		return "", fmt.Errorf("unknown seat %q", seat)
	}
	return path, nil
}

type testGitRunner struct{}

func (testGitRunner) Git(ctx context.Context, dir string, args ...string) (int, string, error) {
	output, code, err := testGit(ctx, dir, args...)
	return code, output, err
}

func testGit(ctx context.Context, dir string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+testCommitDate)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), 0, nil
	}
	if ctx.Err() != nil {
		return string(output), -1, ctx.Err()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(output), exitErr.ExitCode(), nil
	}
	return string(output), -1, err
}
