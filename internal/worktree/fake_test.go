package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

type fakeReply struct {
	Prefix []string
	Exit   int
	Output string
	Err    error
}

type fakeCall struct {
	Dir  string
	Args []string
}

type fakeRunner struct {
	mu      sync.Mutex
	replies []fakeReply
	calls   []fakeCall
}

func newFakeRunner(replies ...fakeReply) *fakeRunner {
	return &fakeRunner{replies: append([]fakeReply(nil), replies...)}
}

func (f *fakeRunner) Git(_ context.Context, dir string, args ...string) (int, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, fakeCall{Dir: dir, Args: append([]string(nil), args...)})
	for _, reply := range f.replies {
		if len(args) >= len(reply.Prefix) && slices.Equal(args[:len(reply.Prefix)], reply.Prefix) {
			return reply.Exit, reply.Output, reply.Err
		}
	}
	return -1, "", fmt.Errorf("unexpected git argv: %q", args)
}

func (f *fakeRunner) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]fakeCall, len(f.calls))
	for i, call := range f.calls {
		calls[i] = fakeCall{Dir: call.Dir, Args: append([]string(nil), call.Args...)}
	}
	return calls
}

type fakeRepo struct {
	name        string
	root        string
	integration string
}

func (r fakeRepo) Name() string        { return r.name }
func (r fakeRepo) Root() string        { return r.root }
func (r fakeRepo) Integration() string { return r.integration }

func initRepo(t *testing.T) fakeRepo {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README")
	runGit(t, root, "-c", "user.name=Magicite Test", "-c", "user.email=test@example.invalid", "commit", "-m", "initial")
	return fakeRepo{name: "fixture", root: root, integration: "main"}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
