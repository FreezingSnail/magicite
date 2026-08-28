package repo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/connorfranc/magicite/internal/logging"
)

func TestGitRootAndAdmit(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	prober := NewProber()
	got, ok := prober.GitRoot(context.Background(), child)
	want, wantOK := Directory(root)
	if !wantOK || !ok || !SameRoot(got, want) {
		t.Fatalf("GitRoot() = %q, %v; want root equivalent to %q, true", got, ok, want)
	}

	admitted, ok := prober.Admit(context.Background(), root)
	if !ok || admitted != want {
		t.Errorf("Admit() = %q, %v; want %q, true", admitted, ok, want)
	}
	if _, ok := prober.Admit(context.Background(), filepath.Join(root, "child")); ok {
		t.Error("Admit(subdirectory) = true, want false")
	}
}

func TestGitRootOutsideWorktree(t *testing.T) {
	prober := NewProber()
	if root, ok := prober.GitRoot(context.Background(), t.TempDir()); ok || root != "" {
		t.Errorf("GitRoot(outside) = %q, %v; want empty false", root, ok)
	}
}

func TestHasBeadsRequiresDirectory(t *testing.T) {
	root := t.TempDir()
	prober := NewProber()
	if prober.HasBeads(root) {
		t.Error("HasBeads(missing) = true, want false")
	}
	if err := os.WriteFile(filepath.Join(root, ".beads"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if prober.HasBeads(root) {
		t.Error("HasBeads(file) = true, want false")
	}
	if err := os.Remove(filepath.Join(root, ".beads")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !prober.HasBeads(root) {
		t.Error("HasBeads(directory) = false, want true")
	}
}

func TestAdmitLogsOneReasonPerRejection(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	if err := os.Mkdir(filepath.Join(root, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	var events []struct {
		level logging.Level
		kind  string
		field map[string]any
	}
	prober := NewProber()
	prober.Log = func(level logging.Level, kind string, fields map[string]any) {
		events = append(events, struct {
			level logging.Level
			kind  string
			field map[string]any
		}{level, kind, fields})
	}

	if _, ok := prober.Admit(context.Background(), root); ok {
		t.Fatal("Admit(no beads) = true, want false")
	}
	if _, ok := prober.Admit(context.Background(), filepath.Join(root, "child")); ok {
		t.Fatal("Admit(subdirectory) = true, want false")
	}
	if _, ok := prober.Admit(context.Background(), ""); ok {
		t.Fatal("Admit(empty) = true, want false")
	}

	if len(events) != 3 {
		t.Fatalf("events = %d, want 3: %#v", len(events), events)
	}
	wantReasons := []string{"no-beads", "not-worktree-root", "invalid-root"}
	for i, event := range events {
		if event.level != logging.Debug || event.kind != "repo.skip" {
			t.Errorf("event[%d] = %#v, want debug repo.skip", i, event)
		}
		if got := event.field["reason"]; got != wantReasons[i] {
			t.Errorf("event[%d] reason = %#v, want %q", i, got, wantReasons[i])
		}
	}
}

func TestAdmitCancelledContextIsProbeFailure(t *testing.T) {
	var events []map[string]any
	prober := NewProber()
	prober.Git = "not-a-real-git-command"
	prober.Log = func(_ logging.Level, kind string, fields map[string]any) {
		if kind == "repo.skip" {
			events = append(events, fields)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, ok := prober.Admit(ctx, t.TempDir()); ok {
		t.Fatal("Admit(cancelled) = true, want false")
	}
	if len(events) != 1 || events[0]["reason"] != "probe-failed" {
		t.Fatalf("events = %#v, want one probe-failed event", events)
	}
}

func TestAdmitProbeFailureLogsWithoutError(t *testing.T) {
	root := t.TempDir()
	var events []struct {
		level logging.Level
		kind  string
	}
	prober := NewProber()
	prober.Git = filepath.Join(root, "missing-git")
	prober.Log = func(level logging.Level, kind string, _ map[string]any) {
		events = append(events, struct {
			level logging.Level
			kind  string
		}{level, kind})
	}
	if _, ok := prober.Admit(context.Background(), root); ok {
		t.Fatal("Admit(unavailable git) = true, want false")
	}
	want := []struct {
		level logging.Level
		kind  string
	}{{logging.Debug, "repo.skip"}}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("events = %#v, want %#v", events, want)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
