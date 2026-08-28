package repotest

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/connorfranc/magicite/internal/repo"
)

func TestFakeSeedListAndRefresh(t *testing.T) {
	first := testRepo(t, "first", "f")
	second := testRepo(t, "second", "s")
	fake := New(first, repo.Repo{})
	fake.Seed(second)

	got := fake.List(context.Background())
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("List() = %#v, want %#v", got, []repo.Repo{first, second})
	}
	got[0] = repo.Repo{}
	if got := fake.List(context.Background()); got[0] != first {
		t.Errorf("List returned shared records: %#v", got)
	}

	got = fake.Refresh(context.Background())
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Errorf("Refresh() = %#v, want %#v", got, []repo.Repo{first, second})
	}
	if got := fake.Refreshes(); got != 1 {
		t.Errorf("Refreshes() = %d, want 1", got)
	}
	fake.Invalidate()
	if got := fake.Invalidations(); got != 1 {
		t.Errorf("Invalidations() = %d, want 1", got)
	}
}

func TestFakeDelegatesLookup(t *testing.T) {
	parent := t.TempDir()
	nested := filepath.Join(parent, "nested")
	first := repo.Repo{Root: parent + string(filepath.Separator), Name: "first", Prefix: "fleet", Branch: "main"}
	second := repo.Repo{Root: nested + string(filepath.Separator), Name: "second", Prefix: "fleet-2az", Branch: "main"}
	fake := New(first, second)
	ctx := context.Background()

	if got, err := fake.Get(ctx, "second"); err != nil || got != second {
		t.Errorf("Get() = %#v, %v; want %#v, nil", got, err, second)
	}
	if got, err := fake.ForBead(ctx, "fleet-2az-10"); err != nil || got != second {
		t.Errorf("ForBead() = %#v, %v; want %#v, nil", got, err, second)
	}
	if got, err := fake.Current(ctx, nested); err != nil || got != second {
		t.Errorf("Current() = %#v, %v; want %#v, nil", got, err, second)
	}

	mapped := repo.Repo{Name: "mapped"}
	fake.SetCurrent("not-a-directory", mapped)
	if got, err := fake.Current(ctx, "not-a-directory"); err != nil || got != mapped {
		t.Errorf("mapped Current() = %#v, %v; want %#v, nil", got, err, mapped)
	}
}

func TestFakeMissesAreTyped(t *testing.T) {
	fake := New()
	ctx := context.Background()
	for _, test := range []struct {
		name  string
		call  func() (repo.Repo, error)
		query string
	}{
		{name: "get", call: func() (repo.Repo, error) { return fake.Get(ctx, "missing") }, query: "missing"},
		{name: "current", call: func() (repo.Repo, error) { return fake.Current(ctx, "missing") }, query: "missing"},
		{name: "bead", call: func() (repo.Repo, error) { return fake.ForBead(ctx, "missing-1") }, query: "missing-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.call()
			if got != (repo.Repo{}) || !repo.IsNotFound(err) {
				t.Fatalf("result = %#v, %v; want zero and not found", got, err)
			}
			var notFound *repo.NotFoundError
			if !errors.As(err, &notFound) || notFound.Query != test.query {
				t.Errorf("error = %v, want NotFoundError for %q", err, test.query)
			}
		})
	}
}

func TestFakeConcurrentUse(t *testing.T) {
	first := repo.Repo{Root: "/one/", Name: "one", Prefix: "one", Branch: "main"}
	second := repo.Repo{Root: "/two/", Name: "two", Prefix: "two", Branch: "main"}
	fake := New(first)
	ctx := context.Background()
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			for range 100 {
				fake.Seed(second)
				fake.SetCurrent("dir", repo.Repo{Name: "current"})
				fake.List(ctx)
				fake.Refresh(ctx)
				fake.Invalidate()
				fake.Get(ctx, "one")
				fake.Current(ctx, "dir")
				fake.ForBead(ctx, "one-1")
				fake.Refreshes()
				fake.Invalidations()
			}
		})
	}
	group.Wait()
}

func testRepo(t *testing.T, name, prefix string) repo.Repo {
	t.Helper()
	return repo.Repo{
		Root:   t.TempDir() + string(filepath.Separator),
		Name:   name,
		Prefix: prefix,
		Branch: "main",
	}
}
