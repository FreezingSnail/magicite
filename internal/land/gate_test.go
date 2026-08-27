package land

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateFuncOverridesRunner(t *testing.T) {
	called := false
	pipeline, err := New(Options{
		Workspace: &fakeWorkspace{},
		Runner:    newFakeRunner(),
		Gate:      []string{"missing-gate"},
		GateFunc: func(context.Context, *Context) (int, error) {
			called = true
			return 0, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := pipeline.gate(context.Background(), &Context{Branch: "ifrit"}); err != nil {
		t.Fatalf("gate() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("GateFunc was not called")
	}
}

func TestGateRefusesNonZeroOnce(t *testing.T) {
	var logs []string
	pipeline, err := New(Options{
		Workspace: &fakeWorkspace{},
		Runner:    newFakeRunner(),
		GateFunc: func(context.Context, *Context) (int, error) {
			return 7, nil
		},
		Log: func(level, message string) {
			logs = append(logs, level+": "+message)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = pipeline.gate(context.Background(), &Context{Branch: "ifrit"})
	if !errors.Is(err, ErrGateFailed) {
		t.Fatalf("gate() error = %v, want ErrGateFailed", err)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "ifrit") || !strings.Contains(logs[0], "7") {
		t.Errorf("logs = %q, want one branch/status warning", logs)
	}
}

func TestGateFuncErrorFailsEvenOnZero(t *testing.T) {
	gateErr := errors.New("gate injection failed")
	pipeline, err := New(Options{
		Workspace: &fakeWorkspace{},
		Runner:    newFakeRunner(),
		GateFunc: func(context.Context, *Context) (int, error) {
			return 0, gateErr
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = pipeline.gate(context.Background(), &Context{Branch: "ifrit"})
	if !errors.Is(err, ErrGateFailed) || !errors.Is(err, gateErr) {
		t.Fatalf("gate() error = %v, want ErrGateFailed and injection error", err)
	}
}

func TestRunGateUsesConfiguredArgvAndWorktree(t *testing.T) {
	worktree := t.TempDir()
	output := filepath.Join(t.TempDir(), "cwd")
	pipeline, err := New(Options{
		Workspace: &fakeWorkspace{},
		Runner:    newFakeRunner(),
		Gate: []string{
			os.Args[0], "-test.run=^TestGateHelperProcess$", "--",
			"--gate-helper", output,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	status, runErr := pipeline.runGate(context.Background(), &Context{Worktree: worktree})
	if runErr != nil {
		t.Fatalf("runGate() error = %v, want nil", runErr)
	}
	if status != 7 {
		t.Fatalf("runGate() status = %d, want 7", status)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err = filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(contents)); got != worktree {
		t.Errorf("gate working directory = %q, want %q", got, worktree)
	}
}

func TestGateHelperProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--gate-helper" || i+1 >= len(os.Args) {
			continue
		}
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(os.Args[i+1], []byte(fmt.Sprintf("%s\n", cwd)), 0o600); err != nil {
			os.Exit(3)
		}
		os.Exit(7)
	}
}

func TestRunGateRefusesSpawnFailure(t *testing.T) {
	pipeline, err := New(Options{
		Workspace: &fakeWorkspace{},
		Runner:    newFakeRunner(),
		Gate:      []string{"definitely-not-a-program"},
	})
	if err != nil {
		t.Fatal(err)
	}

	status, runErr := pipeline.runGate(context.Background(), &Context{Worktree: t.TempDir()})
	if status != -1 || runErr == nil {
		t.Fatalf("runGate() = %d, %v; want -1 and lookup error", status, runErr)
	}
}

func TestGateRejectsEmptyConfiguredGate(t *testing.T) {
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}
	pipeline.gateArgv = nil

	err = pipeline.gate(context.Background(), &Context{Branch: "ifrit"})
	if !errors.Is(err, ErrGateFailed) {
		t.Fatalf("gate() error = %v, want ErrGateFailed", err)
	}
}
