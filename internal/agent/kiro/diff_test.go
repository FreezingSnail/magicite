package kiro

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/agent/conformance"
)

func TestDiffReportsAllGitStates(t *testing.T) {
	workdir := gitWorkdir(t)
	writeFile(t, filepath.Join(workdir, "staged.txt"), "staged change\n")
	runGit(t, workdir, "add", "staged.txt")

	adapter := New(Options{Executable: conformance.FakeCLI(t)})
	handle, err := adapter.Run(context.Background(), agent.RunSpec{
		Workdir: workdir, Model: "fake-model", Plan: conformance.ScenarioOK,
	})
	if err != nil {
		t.Fatal(err)
	}
	await(t, adapter, handle, agent.StatusCompleted)
	diffs, err := adapter.Diff(context.Background(), handle)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"tracked.txt": modified, "staged.txt": staged, "untracked.txt": untracked}
	for _, diff := range diffs {
		if status, ok := want[diff.File]; ok {
			if diff.Status != status {
				t.Errorf("%s status = %q, want %q", diff.File, diff.Status, status)
			}
			if diff.Patch == "" {
				t.Errorf("%s patch empty", diff.File)
			}
			delete(want, diff.File)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing diffs %v", want)
	}
}
