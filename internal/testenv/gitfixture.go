package testenv

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	magicexec "github.com/FreezingSnail/magicite/internal/exec"
)

const (
	fixtureGitName  = "Magicite Fixture"
	fixtureGitEmail = "fixture@magicite.test"
)

// Repo is a deterministic Git repository rooted in a test environment.
type Repo struct {
	Name string
	Root string

	t        *testing.T
	env      *Env
	nextDate time.Time
}

// Trailer is one commit-message trailer in message order.
type Trailer struct {
	Key   string
	Value string
}

// NewRepo creates name below env.Root with an initial commit on main.
func NewRepo(t *testing.T, env *Env, name string) *Repo {
	t.Helper()
	if !fixturePathPart(name) {
		t.Fatalf("invalid fixture repository name %q", name)
	}

	repo := &Repo{
		Name:     name,
		Root:     filepath.Join(env.Root, name),
		t:        t,
		env:      env,
		nextDate: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	repo.gitAt(env.Root, "init", "--quiet", "--initial-branch=main", repo.Root)
	repo.git("config", "user.name", fixtureGitName)
	repo.git("config", "user.email", fixtureGitEmail)
	repo.Commit("initial", nil)
	return repo
}

// Commit writes files and creates a commit with msg. It returns its SHA.
func (r *Repo) Commit(msg string, files map[string]string) string {
	r.t.Helper()

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		file := filepath.Join(r.Root, path)
		if !fixturePath(r.Root, file) {
			r.t.Fatalf("fixture file %q escapes repository %q", path, r.Root)
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			r.t.Fatalf("create fixture directory for %q: %v", path, err)
		}
		if err := os.WriteFile(file, []byte(files[path]), 0o644); err != nil {
			r.t.Fatalf("write fixture file %q: %v", path, err)
		}
	}
	if len(paths) > 0 {
		args := append([]string{"add", "--"}, paths...)
		r.git(args...)
	}
	r.gitWithDate("commit", "--quiet", "--allow-empty", "-m", msg)
	return r.Head("HEAD")
}

// Branch creates name at from.
func (r *Repo) Branch(name, from string) {
	r.t.Helper()
	r.git("branch", name, from)
}

// Checkout switches the primary worktree to name.
func (r *Repo) Checkout(name string) {
	r.t.Helper()
	r.git("checkout", "--quiet", name)
}

// Worktree adds branch at env.Root/seat and returns that path.
func (r *Repo) Worktree(seat, branch string) string {
	r.t.Helper()
	if !fixturePathPart(seat) {
		r.t.Fatalf("invalid fixture worktree seat %q", seat)
	}
	root := filepath.Join(r.env.Root, seat)
	r.git("worktree", "add", "--quiet", root, branch)
	return root
}

// Head resolves ref to a SHA.
func (r *Repo) Head(ref string) string {
	r.t.Helper()
	return r.git("rev-parse", "--verify", ref+"^{commit}")
}

// Log returns commit subjects from ref, newest first.
func (r *Repo) Log(ref string) []string {
	r.t.Helper()
	output := strings.TrimSuffix(r.git("log", "--format=%s", ref), "\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

// Trailers returns commit trailers from ref without collapsing duplicate keys.
func (r *Repo) Trailers(ref string) []Trailer {
	r.t.Helper()
	output := strings.TrimSuffix(r.git("log", "-1", "--format=%(trailers:only,unfold)", ref), "\n")
	if output == "" {
		return nil
	}

	trailers := make([]Trailer, 0)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			r.t.Fatalf("parse git trailer %q", line)
		}
		trailers = append(trailers, Trailer{Key: key, Value: strings.TrimSpace(value)})
	}
	return trailers
}

// Parents returns SHA's direct parents in Git order.
func (r *Repo) Parents(sha string) []string {
	r.t.Helper()
	output := r.git("show", "--no-patch", "--format=%P", sha)
	if output == "" {
		return nil
	}
	return strings.Fields(output)
}

// Linear reports whether every reachable commit from ref has at most one parent.
func (r *Repo) Linear(ref string) bool {
	r.t.Helper()
	for _, line := range strings.Split(strings.TrimSuffix(r.git("rev-list", "--parents", ref), "\n"), "\n") {
		if len(strings.Fields(line)) > 2 {
			return false
		}
	}
	return true
}

// Exists reports whether ref resolves to an object.
func (r *Repo) Exists(ref string) bool {
	r.t.Helper()
	_, _, exitCode, runErr := magicexec.RunEnv(context.Background(), r.Root, r.env.Env(), "git", "rev-parse", "--verify", "--quiet", ref)
	if runErr != nil && exitCode == -1 {
		r.t.Fatalf("start git rev-parse for %q: %v", ref, runErr)
	}
	return runErr == nil && exitCode == 0
}

func (r *Repo) gitWithDate(args ...string) string {
	r.t.Helper()
	env := append(r.env.Env(), "GIT_AUTHOR_DATE="+r.nextDate.Format(time.RFC3339), "GIT_COMMITTER_DATE="+r.nextDate.Format(time.RFC3339))
	output := r.gitWithEnv(r.Root, env, args...)
	r.nextDate = r.nextDate.Add(time.Second)
	return output
}

func (r *Repo) git(args ...string) string {
	r.t.Helper()
	return r.gitWithEnv(r.Root, r.env.Env(), args...)
}

func (r *Repo) gitAt(dir string, args ...string) string {
	r.t.Helper()
	return r.gitWithEnv(dir, r.env.Env(), args...)
}

func (r *Repo) gitWithEnv(dir string, env []string, args ...string) string {
	r.t.Helper()
	stdout, stderr, exitCode, runErr := magicexec.RunEnv(context.Background(), dir, env, "git", args...)
	if runErr != nil || exitCode != 0 {
		r.t.Fatalf("git %s in %q: exit %d, error %v\nstderr:\n%s", strings.Join(args, " "), dir, exitCode, runErr, stderr)
	}
	return strings.TrimSuffix(string(stdout), "\n")
}

func fixturePathPart(part string) bool {
	return part != "" && part != "." && filepath.Base(part) == part
}

func fixturePath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
