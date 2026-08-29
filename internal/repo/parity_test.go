package repo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/parity"
	"github.com/FreezingSnail/magicite/internal/testenv"
)

type repoParityFixture struct {
	first, second *testenv.Repo
}

func newRepoParityFixture(t *testing.T) repoParityFixture {
	t.Helper()
	env := testenv.New(t)
	first := testenv.NewRepo(t, env, "alpha")
	second := testenv.NewRepo(t, env, "beta")
	for _, fixture := range []*testenv.Repo{first, second} {
		if err := os.Mkdir(filepath.Join(fixture.Root, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repoParityFixture{first: first, second: second}
}

func (f repoParityFixture) record(t *testing.T, fixture *testenv.Repo, name, prefix string) Repo {
	t.Helper()
	record, ok := Make(fixture.Root, name, prefix, "main")
	if !ok {
		t.Fatal("Make fixture record")
	}
	return record
}

func TestMaduinRepoParity(t *testing.T) {
	fixture := newRepoParityFixture(t)
	bindings := parity.NewBindings(t, "TestMaduinRepoParity")
	bindings.Bind("maduin-test-repo-admits-git-beads-worktree-roots", func(t *testing.T) {
		registry := NewWith(Options{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{fixture.first.Root}}, Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return nil, nil }}})
		records := registry.Refresh(context.Background())
		if len(records) != 1 || !SameRoot(records[0].Root, fixture.first.Root) {
			t.Fatalf("Refresh() = %#v", records)
		}
	})
	bindings.Bind("maduin-test-repo-falls-back-to-ambient-repository", func(t *testing.T) {
		registry := NewWith(Options{Repos: config.ReposConfig{Discover: "project"}, Dir: fixture.first.Root, Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return nil, nil }}})
		records := registry.Refresh(context.Background())
		if len(records) == 0 || !SameRoot(records[0].Root, fixture.first.Root) {
			t.Fatalf("ambient Refresh() = %#v", records)
		}
	})
	bindings.Bind("maduin-test-repo-fallback-respects-explicit-and-filters", func(t *testing.T) {
		finder := Finder{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{fixture.first.Root}, Exclude: []string{"alpha"}}}
		if got := finder.Candidates(context.Background()); len(got) != 0 {
			t.Fatalf("explicit filtered candidates = %#v", got)
		}
	})
	bindings.Bind("maduin-test-repo-include-filter", func(t *testing.T) {
		finder := Finder{Repos: config.ReposConfig{Include: []string{"beta"}}}
		got := finder.Filter([]string{fixture.first.Root, fixture.second.Root})
		if !reflect.DeepEqual(got, []string{fixture.second.Root}) {
			t.Fatalf("Filter() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-repo-explicit-roots-ignore-discovery", func(t *testing.T) {
		finder := Finder{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{fixture.second.Root}}}
		if got := finder.Candidates(context.Background()); !reflect.DeepEqual(got, []string{filepath.Clean(fixture.second.Root) + string(filepath.Separator)}) {
			t.Fatalf("Candidates() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-repo-names-prefixes-and-cache-are-stable", func(t *testing.T) {
		registry := NewWith(Options{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{fixture.first.Root, fixture.second.Root}}, Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return nil, nil }}})
		first := registry.List(context.Background())
		second := registry.List(context.Background())
		if len(first) != 2 || !reflect.DeepEqual(first, second) || first[0].Name >= first[1].Name {
			t.Fatalf("cached records = %#v then %#v", first, second)
		}
	})
	bindings.Bind("maduin-test-repo-prefix-sources-validate-and-fall-back", func(t *testing.T) {
		if !ValidPrefix("bd_42") || ValidPrefix("invalid value") {
			t.Fatal("ValidPrefix accepted invalid source or rejected valid source")
		}
	})
	bindings.Bind("maduin-test-repo-malformed-config-and-records-never-signal", func(t *testing.T) {
		record, ok := Make(fixture.first.Root, "", "", "")
		if !ok || record.Name != "alpha" || record.Prefix != "alpha" || record.Branch != "main" {
			t.Fatalf("Make() = %#v, %t", record, ok)
		}
	})
	bindings.Bind("maduin-test-repo-lookups-get-and-bead-routing", func(t *testing.T) {
		alpha := fixture.record(t, fixture.first, "alpha", "alpha")
		beta := fixture.record(t, fixture.second, "beta", "beta-long")
		if got, err := GetIn([]Repo{alpha, beta}, fixture.first.Root); err != nil || got != alpha {
			t.Fatalf("GetIn() = %#v, %v", got, err)
		}
		if got, err := ForBeadIn([]Repo{alpha, beta}, "beta-long-9"); err != nil || got != beta {
			t.Fatalf("ForBeadIn() = %#v, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-repo-current-project-then-directory", func(t *testing.T) {
		parent := fixture.record(t, fixture.first, "alpha", "alpha")
		childRoot := filepath.Join(fixture.first.Root, "nested")
		if err := os.MkdirAll(filepath.Join(childRoot, "files"), 0o755); err != nil {
			t.Fatal(err)
		}
		child, ok := Make(childRoot, "nested", "nested", "main")
		if !ok {
			t.Fatal("Make child")
		}
		got, err := CurrentIn([]Repo{parent, child}, filepath.Join(childRoot, "files"))
		if err != nil || got != child {
			t.Fatalf("CurrentIn() = %#v, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-repo-read-completes-and-empty-registry-errors", func(t *testing.T) {
		if _, err := GetIn(nil, "missing"); !IsNotFound(err) {
			t.Fatalf("GetIn(empty) error = %v", err)
		}
	})
	bindings.Bind("maduin-test-multirepo-isolation-workspace-paths-use-distinct-roots", func(t *testing.T) {
		alpha := fixture.record(t, fixture.first, "alpha", "alpha")
		beta := fixture.record(t, fixture.second, "beta", "beta")
		if alpha.Root == beta.Root || !SameRoot(alpha.Root, fixture.first.Root) || !SameRoot(beta.Root, fixture.second.Root) {
			t.Fatalf("repository roots = %#v %#v", alpha, beta)
		}
	})
	bindings.Bind("maduin-test-multirepo-isolation-land-target-and-provenance", func(t *testing.T) {
		alpha := fixture.record(t, fixture.first, "alpha", "a")
		beta := fixture.record(t, fixture.second, "beta", "b")
		if got, err := ForBeadIn([]Repo{alpha, beta}, "a-task"); err != nil || !SameRoot(got.Root, fixture.first.Root) {
			t.Fatalf("ForBeadIn(a-task) = %#v, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-multirepo-isolation-review-drift-stays-in-a", func(t *testing.T) {
		alpha := fixture.record(t, fixture.first, "alpha", "a")
		beta := fixture.record(t, fixture.second, "beta", "b")
		if got, err := ForBeadIn([]Repo{alpha, beta}, "b-fix-1"); err != nil || got != beta {
			t.Fatalf("ForBeadIn(b-fix-1) = %#v, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-multirepo-isolation-failure-warns-once-b-dispatches", func(t *testing.T) {
		logs := 0
		registry := NewWith(Options{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{fixture.second.Root}}, Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return nil, nil }}, Log: func(logging.Level, string, map[string]any) { logs++ }})
		if got := registry.List(context.Background()); len(got) != 1 || logs != 1 {
			t.Fatalf("List() = %#v, logs=%d", got, logs)
		}
	})
	bindings.Bind("maduin-test-multirepo-isolation-global-cap-is-three-sessions", func(t *testing.T) {
		records := Records([]string{fixture.second.Root, fixture.first.Root, fixture.second.Root})
		if len(records) != 2 || records[0].Name >= records[1].Name {
			t.Fatalf("Records() = %#v", records)
		}
	})
	bindings.Bind("maduin-test-multirepo-isolation-cockpit-rows-are-contiguous", func(t *testing.T) {
		records := Records([]string{fixture.second.Root, fixture.first.Root})
		if len(records) != 2 || records[0].Name != "alpha" || records[1].Name != "beta" {
			t.Fatalf("Records() ordering = %#v", records)
		}
	})
	bindings.Run()
}
