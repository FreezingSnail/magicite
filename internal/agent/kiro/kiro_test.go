package kiro

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/agent/conformance"
)

func TestRunRefusesInvalidInputs(t *testing.T) {
	adapter := New(Options{Executable: conformance.FakeCLI(t)})
	for _, spec := range []agent.RunSpec{
		{Workdir: t.TempDir(), Model: "fake/model", Plan: conformance.ScenarioOK},
		{Workdir: t.TempDir(), Model: "fake-model", Plan: " "},
		{Workdir: t.TempDir(), Model: "fake-model", Agent: "../agent", Plan: conformance.ScenarioOK},
	} {
		if _, err := adapter.Run(context.Background(), spec); err == nil {
			t.Errorf("Run(%+v) error = nil", spec)
		}
	}
	if handles := adapter.store.Handles(); len(handles) != 0 {
		t.Fatalf("refused runs left handles %v", handles)
	}
}

func TestOutputStripsANSIAndClassifiesFailure(t *testing.T) {
	adapter := New(Options{Executable: conformance.FakeCLI(t)})
	handle, err := adapter.Run(context.Background(), agent.RunSpec{
		Workdir: gitWorkdir(t), Model: "fake-model", Plan: conformance.ScenarioDenied,
	})
	if err != nil {
		t.Fatal(err)
	}
	await(t, adapter, handle, agent.StatusFailed)
	output, err := adapter.Output(context.Background(), handle)
	if err != nil {
		t.Fatal(err)
	}
	if output != "authentication denied\n" {
		t.Errorf("Output = %q, want stripped denial", output)
	}
}

func gitWorkdir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, "tracked.txt"), "initial\n")
	writeFile(t, filepath.Join(dir, "staged.txt"), "initial\n")
	runGit(t, dir, "add", "tracked.txt", "staged.txt")
	runGit(t, dir, "-c", "user.name=magicite", "-c", "user.email=magicite@example.test", "commit", "-qm", "initial")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func await(t *testing.T, adapter *Adapter, handle agent.Handle, want agent.Status) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := adapter.Complete(context.Background(), handle); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := adapter.Complete(context.Background(), handle)
	t.Fatalf("Complete = (%q, %v), want %q", got, err, want)
}
