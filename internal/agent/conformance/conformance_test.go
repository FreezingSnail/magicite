package conformance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFakeCLIEmulatesOpenCode(t *testing.T) {
	workdir := t.TempDir()
	output := runFake(t, workdir, "run", "--format", "json", ScenarioOK)
	if !strings.Contains(output, `"type":"step_finish"`) || !strings.Contains(output, `"reason":"stop"`) {
		t.Fatalf("run output = %q, want completed NDJSON", output)
	}
	assertFixtureFiles(t, workdir)

	export := runFake(t, workdir, "export", "ses_fake")
	if !strings.Contains(export, `"tracked.txt"`) || !strings.Contains(export, `"untracked.txt"`) {
		t.Fatalf("export output = %q, want fake diff files", export)
	}
}

func TestFakeCLIEmulatesKiro(t *testing.T) {
	workdir := t.TempDir()
	output := runFake(t, workdir, "chat", "--no-interactive", "--model", "fake-model", "--trust-all-tools", ScenarioLimited)
	if !strings.Contains(output, "\x1b[") || !strings.Contains(output, "usage limit exceeded") {
		t.Fatalf("chat output = %q, want ANSI plain text usage limit", output)
	}
	assertFixtureFiles(t, workdir)
}

func runFake(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(FakeCLI(t), args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fake CLI %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func assertFixtureFiles(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"tracked.txt", "untracked.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("fake CLI did not write %s: %v", name, err)
		}
	}
}

func TestProcessAliveHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := processAlive(ctx, 1); err == nil {
		t.Fatal("processAlive canceled context error = nil")
	}
}
