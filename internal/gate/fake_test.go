package gate

import (
	"context"
	"sync"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
)

type fakeCall struct {
	method string
	args   []any
}

type fakeBeads struct {
	mu       sync.Mutex
	calls    []fakeCall
	beads    map[string]bd.Bead
	labels   map[string][]string
	children map[string][]bd.Bead
	queries  map[string][]bd.Bead
	nextID   string

	show         func(context.Context, repo.Repo, string) (bd.Bead, error)
	labelsFor    func(context.Context, repo.Repo, string) ([]string, error)
	epicChildren func(context.Context, repo.Repo, string) ([]bd.Bead, error)
	query        func(context.Context, repo.Repo, string) ([]bd.Bead, error)
	create       func(context.Context, repo.Repo, bd.CreateRequest) (string, error)
	comment      func(context.Context, repo.Repo, string, string) error
	close        func(context.Context, repo.Repo, string, string) error
}

var _ Beads = (*fakeBeads)(nil)

func (f *fakeBeads) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method: method, args: append([]any(nil), args...)})
}

func (f *fakeBeads) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func (f *fakeBeads) Show(ctx context.Context, r repo.Repo, id string) (bd.Bead, error) {
	f.record("Show", r, id)
	if f.show != nil {
		return f.show(ctx, r, id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.beads[id], nil
}

func (f *fakeBeads) Labels(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	f.record("Labels", r, id)
	if f.labelsFor != nil {
		return f.labelsFor(ctx, r, id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.labels[id]...), nil
}

func (f *fakeBeads) EpicChildren(ctx context.Context, r repo.Repo, epic string) ([]bd.Bead, error) {
	f.record("EpicChildren", r, epic)
	if f.epicChildren != nil {
		return f.epicChildren(ctx, r, epic)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bd.Bead(nil), f.children[epic]...), nil
}

func (f *fakeBeads) Query(ctx context.Context, r repo.Repo, q string) ([]bd.Bead, error) {
	f.record("Query", r, q)
	if f.query != nil {
		return f.query(ctx, r, q)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bd.Bead(nil), f.queries[q]...), nil
}

func (f *fakeBeads) Create(ctx context.Context, r repo.Repo, req bd.CreateRequest) (string, error) {
	f.record("Create", r, req)
	if f.create != nil {
		return f.create(ctx, r, req)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextID == "" {
		f.nextID = "created"
	}
	if f.beads == nil {
		f.beads = make(map[string]bd.Bead)
	}
	f.beads[f.nextID] = bd.Bead{ID: f.nextID, Title: req.Title, Description: req.Body, Design: req.Design, Parent: req.Parent}
	return f.nextID, nil
}

func (f *fakeBeads) Comment(ctx context.Context, r repo.Repo, id, text string) error {
	f.record("Comment", r, id, text)
	if f.comment != nil {
		return f.comment(ctx, r, id, text)
	}
	return nil
}

func (f *fakeBeads) Close(ctx context.Context, r repo.Repo, id, reason string) error {
	f.record("Close", r, id, reason)
	if f.close != nil {
		return f.close(ctx, r, id, reason)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if bead, ok := f.beads[id]; ok {
		bead.Status = "closed"
		f.beads[id] = bead
	}
	return nil
}

type fakeGit struct {
	mu     sync.Mutex
	calls  [][]string
	output func(context.Context, repo.Repo, ...string) (int, string, error)
}

var _ Git = (*fakeGit)(nil)

func (f *fakeGit) Output(ctx context.Context, r repo.Repo, args ...string) (int, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string(nil), args...))
	f.mu.Unlock()
	if f.output != nil {
		return f.output(ctx, r, args...)
	}
	return 0, "", nil
}

func (f *fakeGit) Calls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([][]string, len(f.calls))
	for i := range f.calls {
		calls[i] = append([]string(nil), f.calls[i]...)
	}
	return calls
}

type fakeRepos struct {
	mu      sync.Mutex
	calls   []string
	records map[string]repo.Repo
	get     func(string) (repo.Repo, bool)
}

var _ Repos = (*fakeRepos)(nil)

func (f *fakeRepos) Get(name string) (repo.Repo, bool) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	if f.get != nil {
		get := f.get
		f.mu.Unlock()
		return get(name)
	}
	r, ok := f.records[name]
	f.mu.Unlock()
	return r, ok
}

func (f *fakeRepos) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
