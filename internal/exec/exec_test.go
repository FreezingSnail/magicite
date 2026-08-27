package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunHelperProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator == -1 || len(os.Args) < separator+2 {
		return
	}

	switch os.Args[separator+1] {
	case "emit":
		argument := os.Args[separator+2]
		fmt.Fprintf(os.Stdout, "out:%s", argument)
		fmt.Fprintf(os.Stderr, "err:%s", argument)
		os.Exit(7)
	case "pwd":
		wd, err := os.Getwd()
		if err != nil {
			os.Exit(2)
		}
		fmt.Fprint(os.Stdout, wd)
		os.Exit(0)
	case "environment":
		fmt.Fprint(os.Stdout, os.Getenv("MAGICITE_EXEC_TEST_ENV"))
		os.Exit(0)
	case "wait":
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}
}

func helperCommand(action string, args ...string) (string, []string) {
	return os.Args[0], append([]string{"-test.run=^TestRunHelperProcess$", "--", action}, args...)
}

func TestRunCapturesArgvOutputAndExitCode(t *testing.T) {
	t.Parallel()

	argument := `literal; $(not-a-command) && "quoted" | <input>`
	name, args := helperCommand("emit", argument)
	stdout, stderr, exitCode, runErr := Run(context.Background(), ".", name, args...)

	if got, want := string(stdout), "out:"+argument; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got, want := string(stderr), "err:"+argument; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if exitCode != 7 {
		t.Errorf("exit code = %d, want 7", exitCode)
	}
	var typedErr *Error
	if !errors.As(runErr, &typedErr) {
		t.Errorf("error = %T, want *Error", runErr)
	}
}

func TestRunSetsDirectoryAndDoesNotInheritEnvironment(t *testing.T) {
	t.Setenv("MAGICITE_EXEC_TEST_ENV", "inherited")
	workDir := t.TempDir()

	name, args := helperCommand("pwd")
	stdout, _, exitCode, runErr := Run(context.Background(), workDir, name, args...)
	if runErr != nil || exitCode != 0 {
		t.Fatalf("Run() = exit %d, error %v", exitCode, runErr)
	}
	want, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(stdout); got != want {
		t.Errorf("working directory = %q, want %q", got, want)
	}

	name, args = helperCommand("environment")
	stdout, _, exitCode, runErr = Run(context.Background(), ".", name, args...)
	if runErr != nil || exitCode != 0 {
		t.Fatalf("Run() = exit %d, error %v", exitCode, runErr)
	}
	if got := string(stdout); got != "" {
		t.Errorf("inherited environment = %q, want empty", got)
	}
}

func TestRunHonorsCancellation(t *testing.T) {
	name, args := helperCommand("wait")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan *Error, 1)
	go func() {
		_, _, _, runErr := Run(ctx, ".", name, args...)
		result <- runErr
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Errorf("error = %v, want context cancellation", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
