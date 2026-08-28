package exec

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSessionHelperProcess(t *testing.T) {
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
	case "stream":
		fmt.Fprintln(os.Stdout, "ready")
		fmt.Fprintln(os.Stderr, "warning")
		time.Sleep(5 * time.Second)
		syscall.Exit(0)
	case "exit":
		fmt.Fprint(os.Stdout, os.Args[separator+2])
		syscall.Exit(7)
	case "pwd":
		wd, err := os.Getwd()
		if err != nil {
			syscall.Exit(2)
		}
		fmt.Fprint(os.Stdout, wd)
		syscall.Exit(0)
	case "environment":
		fmt.Fprint(os.Stdout, os.Getenv("MAGICITE_SESSION_TEST_ENV"))
		syscall.Exit(0)
	case "parent":
		child := osexec.Command(os.Args[0], "-test.run=^TestSessionHelperProcess$", "--", "grandchild")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			syscall.Exit(2)
		}
		fmt.Fprintln(os.Stdout, child.Process.Pid)
		time.Sleep(5 * time.Second)
		syscall.Exit(0)
	case "grandchild":
		time.Sleep(5 * time.Second)
		syscall.Exit(0)
	}
}

func sessionHelper(action string, args ...string) (string, []string) {
	return os.Args[0], append([]string{"-test.run=^TestSessionHelperProcess$", "--", action}, args...)
}

func startSession(t *testing.T, ctx context.Context, action string, args ...string) *Session {
	t.Helper()
	name, commandArgs := sessionHelper(action, args...)
	session, err := Start(ctx, Spec{Dir: ".", Name: name, Args: commandArgs})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestStartValidatesProgramAndWorkingDirectory(t *testing.T) {
	t.Parallel()

	if _, err := Start(context.Background(), Spec{Dir: "."}); err == nil {
		t.Fatal("Start accepted empty program")
	}
	if _, err := Start(context.Background(), Spec{Dir: "exec.go", Name: os.Args[0]}); err == nil {
		t.Fatal("Start accepted file working directory")
	}
}

func TestSessionStreamsOutputAndWaitsIdempotently(t *testing.T) {
	session := startSession(t, context.Background(), "stream")
	if session.PID() <= 0 {
		t.Fatalf("PID = %d, want positive", session.PID())
	}

	stdout := bufio.NewReader(session.Stdout())
	if line, err := stdout.ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("stdout = (%q, %v), want ready", line, err)
	}
	stderr := bufio.NewReader(session.Stderr())
	if line, err := stderr.ReadString('\n'); err != nil || line != "warning\n" {
		t.Fatalf("stderr = (%q, %v), want warning", line, err)
	}

	if err := session.Terminate(10 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	code, err := session.Wait()
	if err != nil || code == 0 {
		t.Fatalf("Wait() = (%d, %v), want non-zero, nil", code, err)
	}
	if repeatCode, repeatErr := session.Wait(); repeatErr != nil || repeatCode != code {
		t.Fatalf("second Wait() = (%d, %v), want (%d, nil)", repeatCode, repeatErr, code)
	}
	select {
	case <-session.Done():
	case <-time.After(time.Second):
		t.Fatal("Done did not close")
	}
	if _, err := stdout.ReadString('\n'); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout after Wait error = %v, want EOF", err)
	}
}

func TestSessionPassesArgvDirectoryAndExplicitEnvironment(t *testing.T) {
	argument := `literal; $(not-a-command) && "quoted" | <input>`
	session := startSession(t, context.Background(), "exit", argument)
	output, err := io.ReadAll(session.Stdout())
	if err != nil {
		t.Fatal(err)
	}
	code, err := session.Wait()
	if err != nil || code != 7 || string(output) != argument {
		t.Fatalf("argv session = (%d, %v, %q), want (7, nil, %q)", code, err, output, argument)
	}

	workDir := ".."
	name, args := sessionHelper("pwd")
	session, err = Start(context.Background(), Spec{Dir: workDir, Name: name, Args: args})
	if err != nil {
		t.Fatal(err)
	}
	output, err = io.ReadAll(session.Stdout())
	if err != nil {
		t.Fatal(err)
	}
	if code, err = session.Wait(); err != nil || code != 0 {
		t.Fatalf("pwd Wait() = (%d, %v)", code, err)
	}
	wantDir, err := filepath.Abs(workDir)
	if err != nil {
		t.Fatal(err)
	}
	wantDir, err = filepath.EvalSymlinks(wantDir)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != wantDir {
		t.Fatalf("working directory = %q, want %q", output, wantDir)
	}

	t.Setenv("MAGICITE_SESSION_TEST_ENV", "inherited")
	name, args = sessionHelper("environment")
	session, err = Start(context.Background(), Spec{Dir: ".", Name: name, Args: args})
	if err != nil {
		t.Fatal(err)
	}
	output, _ = io.ReadAll(session.Stdout())
	_, _ = session.Wait()
	if string(output) != "" {
		t.Fatalf("inherited environment = %q, want empty", output)
	}
	session, err = Start(context.Background(), Spec{Dir: ".", Name: name, Args: args, Env: []string{"MAGICITE_SESSION_TEST_ENV=explicit"}})
	if err != nil {
		t.Fatal(err)
	}
	output, _ = io.ReadAll(session.Stdout())
	_, _ = session.Wait()
	if string(output) != "explicit" {
		t.Fatalf("explicit environment = %q, want explicit", output)
	}
}

func TestSessionTerminateAndCancellationKillDescendants(t *testing.T) {
	for _, cancellation := range []bool{false, true} {
		t.Run(strconv.FormatBool(cancellation), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			session := startSession(t, ctx, "parent")
			stdout := bufio.NewReader(session.Stdout())
			line, err := stdout.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			childPID, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil {
				t.Fatal(err)
			}

			if cancellation {
				cancel()
			} else if err := session.Terminate(10 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
			code, err := session.Wait()
			if err != nil || code == 0 {
				t.Fatalf("Wait() = (%d, %v), want non-zero, nil", code, err)
			}
			if alive, err := signalGroup(childPID, 0); err != nil {
				t.Fatal(err)
			} else if alive {
				t.Fatalf("grandchild %d survived", childPID)
			}
		})
	}
}

func TestSessionTerminateAfterExitIsNoop(t *testing.T) {
	session := startSession(t, context.Background(), "pwd")
	if _, err := io.ReadAll(session.Stdout()); err != nil {
		t.Fatal(err)
	}
	if code, err := session.Wait(); err != nil || code != 0 {
		t.Fatalf("Wait() = (%d, %v)", code, err)
	}
	if err := session.Terminate(time.Millisecond); err != nil {
		t.Fatal(err)
	}
}
