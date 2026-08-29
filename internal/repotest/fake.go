// Package repotest provides an in-memory repo.Lookup for consumer tests.
package repotest

import (
	"context"
	"sync"

	"github.com/FreezingSnail/magicite/internal/repo"
)

// Fake is a synchronized in-memory repository lookup.
type Fake struct {
	mu            sync.Mutex
	repos         []repo.Repo
	current       map[string]repo.Repo
	refreshes     int
	invalidations int
}

var _ repo.Lookup = (*Fake)(nil)

// New creates a fake seeded with valid repository records.
func New(repos ...repo.Repo) *Fake {
	f := &Fake{current: make(map[string]repo.Repo)}
	f.Seed(repos...)
	return f
}

// Seed appends valid repository records in order.
func (f *Fake) Seed(repos ...repo.Repo) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, record := range repos {
		if record.Valid() {
			f.repos = append(f.repos, record)
		}
	}
}

// SetCurrent maps dir exactly to r.
func (f *Fake) SetCurrent(dir string, r repo.Repo) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.current == nil {
		f.current = make(map[string]repo.Repo)
	}
	f.current[dir] = r
}

// Refreshes reports Refresh calls.
func (f *Fake) Refreshes() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.refreshes
}

// Invalidations reports Invalidate calls.
func (f *Fake) Invalidations() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.invalidations
}

// List returns a copy of seeded records.
func (f *Fake) List(context.Context) []repo.Repo {
	f.mu.Lock()
	defer f.mu.Unlock()

	return copyRepos(f.repos)
}

// Refresh counts the call and returns a copy of seeded records.
func (f *Fake) Refresh(context.Context) []repo.Repo {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.refreshes++
	return copyRepos(f.repos)
}

// Invalidate counts the call.
func (f *Fake) Invalidate() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.invalidations++
}

// Get resolves nameOrRoot among seeded records.
func (f *Fake) Get(ctx context.Context, nameOrRoot string) (repo.Repo, error) {
	return repo.GetIn(f.List(ctx), nameOrRoot)
}

// Current resolves an exact mapping, then seeded records.
func (f *Fake) Current(ctx context.Context, dir string) (repo.Repo, error) {
	f.mu.Lock()
	record, ok := f.current[dir]
	repos := copyRepos(f.repos)
	f.mu.Unlock()
	if ok {
		return record, nil
	}
	return repo.CurrentIn(repos, dir)
}

// ForBead resolves id among seeded records.
func (f *Fake) ForBead(ctx context.Context, id string) (repo.Repo, error) {
	return repo.ForBeadIn(f.List(ctx), id)
}

func copyRepos(repos []repo.Repo) []repo.Repo {
	return append([]repo.Repo(nil), repos...)
}
