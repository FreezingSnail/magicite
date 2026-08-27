package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
)

func TestFinderCandidatesExplicitFiltersAndDeduplicates(t *testing.T) {
	one := filepath.Join(t.TempDir(), "one")
	two := filepath.Join(t.TempDir(), "two")
	oneDirectory := mustDirectory(t, one)
	twoDirectory := mustDirectory(t, two)

	finder := Finder{Repos: config.ReposConfig{
		Discover: "explicit",
		Roots:    []string{one, two, one},
		Include:  []string{oneDirectory, filepath.Base(two)},
		Exclude:  []string{stringsTrimSuffix(twoDirectory)},
	}}
	if got, want := finder.Candidates(context.Background()), []string{oneDirectory}; !reflect.DeepEqual(got, want) {
		t.Errorf("Candidates() = %#v, want %#v", got, want)
	}
}

func TestFinderCandidatesProjectScansImmediateDirectories(t *testing.T) {
	parent := t.TempDir()
	ambient := filepath.Join(parent, "ambient")
	for _, name := range []string{"ambient", "alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(parent, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "alpha"), filepath.Join(parent, "linked")); err != nil {
		t.Fatal(err)
	}
	runGit(t, ambient, "init", "--quiet")

	finder := Finder{Repos: config.ReposConfig{Discover: "project"}, Probe: NewProber(), Dir: ambient}
	ambientRoot, ok := finder.Probe.GitRoot(context.Background(), ambient)
	if !ok {
		t.Fatal("GitRoot(ambient) failed")
	}
	scanParent := filepath.Dir(stringsTrimSuffix(ambientRoot))
	want := []string{
		ambientRoot,
		mustDirectory(t, filepath.Join(scanParent, "alpha")),
		mustDirectory(t, filepath.Join(scanParent, "beta")),
	}
	if got := finder.Candidates(context.Background()); !reflect.DeepEqual(got, want) {
		t.Errorf("Candidates() = %#v, want %#v", got, want)
	}
}

func TestFinderCandidatesProjectCapsScan(t *testing.T) {
	parent := t.TempDir()
	ambient := filepath.Join(parent, "z-ambient")
	if err := os.Mkdir(ambient, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxScanEntries+1; i++ {
		if err := os.Mkdir(filepath.Join(parent, fmt.Sprintf("entry-%03d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, ambient, "init", "--quiet")

	finder := Finder{Repos: config.ReposConfig{Discover: "project"}, Probe: NewProber(), Dir: ambient}
	ambientRoot, ok := finder.Probe.GitRoot(context.Background(), ambient)
	if !ok {
		t.Fatal("GitRoot(ambient) failed")
	}
	scanParent := filepath.Dir(stringsTrimSuffix(ambientRoot))
	got := finder.Candidates(context.Background())
	if len(got) != maxScanEntries+1 {
		t.Fatalf("len(Candidates()) = %d, want %d", len(got), maxScanEntries+1)
	}
	if got[0] != ambientRoot {
		t.Errorf("Candidates()[0] = %q, want ambient root", got[0])
	}
	if got[len(got)-1] != mustDirectory(t, filepath.Join(scanParent, fmt.Sprintf("entry-%03d", maxScanEntries-1))) {
		t.Errorf("last candidate = %q, want capped final entry", got[len(got)-1])
	}
}

func TestFinderCandidatesUnknownOrUnusableProjectAreEmpty(t *testing.T) {
	for _, finder := range []Finder{
		{Repos: config.ReposConfig{Discover: "unknown"}},
		{Repos: config.ReposConfig{Discover: "project"}},
	} {
		got := finder.Candidates(context.Background())
		if got == nil || len(got) != 0 {
			t.Errorf("Candidates() = %#v, want non-nil empty slice", got)
		}
	}
}

func TestFinderFilterMatchesCandidateForms(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "candidate")
	directory := mustDirectory(t, candidate)
	forms := []string{candidate, directory, stringsTrimSuffix(directory), filepath.Base(candidate)}
	for _, form := range forms {
		finder := Finder{Repos: config.ReposConfig{Include: []string{form}}}
		if got := finder.Filter([]string{candidate}); !reflect.DeepEqual(got, []string{candidate}) {
			t.Errorf("Filter(include %q) = %#v, want candidate", form, got)
		}
	}
	finder := Finder{Repos: config.ReposConfig{Include: []string{"  "}, Exclude: []string{filepath.Base(candidate)}}}
	if got := finder.Filter([]string{candidate}); len(got) != 0 {
		t.Errorf("Filter(blank include, base exclude) = %#v, want empty", got)
	}
}

func mustDirectory(t *testing.T, path string) string {
	t.Helper()
	directory, ok := Directory(path)
	if !ok {
		t.Fatalf("Directory(%q) failed", path)
	}
	return directory
}

func stringsTrimSuffix(path string) string {
	return path[:len(path)-len(string(filepath.Separator))]
}
