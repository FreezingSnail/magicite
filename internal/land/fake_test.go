package land

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

type fakeReply struct {
	Prefix []string
	Code   int
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
			return reply.Code, reply.Output, reply.Err
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

type workspaceCall struct {
	Method string
	Repo   Repo
	Seat   string
}

type fakeWorkspace struct {
	mu        sync.Mutex
	branch    string
	branchErr error
	path      string
	pathErr   error
	calls     []workspaceCall
}

func (f *fakeWorkspace) Branch(repo Repo, seat string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, workspaceCall{Method: "Branch", Repo: repo, Seat: seat})
	return f.branch, f.branchErr
}

func (f *fakeWorkspace) Path(repo Repo, seat string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, workspaceCall{Method: "Path", Repo: repo, Seat: seat})
	return f.path, f.pathErr
}

func (f *fakeWorkspace) Calls() []workspaceCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]workspaceCall(nil), f.calls...)
}

type fakeRepo struct {
	name        string
	root        string
	integration string
}

func (r fakeRepo) Name() string        { return r.name }
func (r fakeRepo) Root() string        { return r.root }
func (r fakeRepo) Integration() string { return r.integration }
