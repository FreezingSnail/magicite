package repo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/connorfranc/magicite/internal/config"
)

func TestFinderAmbientCandidatesExplicitDoesNotProbe(t *testing.T) {
	finder := Finder{
		Repos: config.ReposConfig{Discover: "explicit"},
		Probe: &Prober{Git: filepath.Join(t.TempDir(), "missing-git")},
	}
	got := finder.AmbientCandidates(context.Background())
	if got == nil || len(got) != 0 {
		t.Fatalf("AmbientCandidates() = %#v, want non-nil empty slice", got)
	}
}

func TestFinderAmbientCandidatesUsesDirectoryAndFilter(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "--quiet")

	finder := Finder{
		Repos: config.ReposConfig{Include: []string{filepath.Base(root)}},
		Probe: NewProber(),
		Dir:   child,
	}
	wantRoot, ok := finder.Probe.GitRoot(context.Background(), child)
	if !ok {
		t.Fatal("GitRoot(child) failed")
	}
	want := []string{wantRoot}
	if got := finder.AmbientCandidates(context.Background()); !reflect.DeepEqual(got, want) {
		t.Fatalf("AmbientCandidates() = %#v, want %#v", got, want)
	}
}

func TestFinderAmbientCandidatesProbeFailureIsEmpty(t *testing.T) {
	finder := Finder{
		Probe: &Prober{Git: filepath.Join(t.TempDir(), "missing-git")},
		Dir:   t.TempDir(),
	}
	got := finder.AmbientCandidates(context.Background())
	if got == nil || len(got) != 0 {
		t.Fatalf("AmbientCandidates() = %#v, want non-nil empty slice", got)
	}
}

func TestFinderAmbientCandidatesDeduplicatesRoots(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })

	finder := Finder{Probe: NewProber()}
	wantRoot, ok := finder.Probe.GitRoot(context.Background(), root)
	if !ok {
		t.Fatal("GitRoot(root) failed")
	}
	got := finder.AmbientCandidates(context.Background())
	want := []string{wantRoot}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AmbientCandidates() = %#v, want %#v", got, want)
	}
}
