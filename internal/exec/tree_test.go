package exec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTreeHelperProcess(t *testing.T) {
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
	case "parent":
		child := osexec.Command(os.Args[0], "-test.run=^TestTreeHelperProcess$", "--", "grandchild")
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
	case "ignore-term":
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		defer signal.Stop(signals)
		fmt.Fprintln(os.Stdout, "ready")
		<-signals
		fmt.Fprintln(os.Stderr, "graceful")
		time.Sleep(5 * time.Second)
		syscall.Exit(0)
	case "exit":
		syscall.Exit(0)
	default:
		syscall.Exit(0)
	}
}

func startTreeHelper(t *testing.T, action string) (*osexec.Cmd, *bufio.Reader, *bytes.Buffer) {
	t.Helper()

	cmd := osexec.Command(os.Args[0], "-test.run=^TestTreeHelperProcess$", "--", action)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, bufio.NewReader(stdout), stderr
}

func TestTerminateKillsDescendants(t *testing.T) {
	cmd, stdout, _ := startTreeHelper(t, "parent")
	line, err := stdout.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Terminate(ctx, cmd.Process); err != nil {
		t.Fatal(err)
	}
	if err := <-wait; err == nil {
		t.Fatal("parent exited successfully after termination")
	}
	if alive, err := signalGroup(childPID, 0); err != nil {
		t.Fatal(err)
	} else if alive {
		t.Fatalf("grandchild %d survived termination", childPID)
	}
}

func TestTerminateEscalatesAfterGracefulSignal(t *testing.T) {
	cmd, stdout, stderr := startTreeHelper(t, "ignore-term")
	ready, err := stdout.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if ready != "ready\n" {
		t.Fatalf("ready = %q, want ready", ready)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()

	if err := Terminate(context.Background(), cmd.Process); err != nil {
		t.Fatal(err)
	}
	if err := <-wait; err == nil {
		t.Fatal("process exited successfully after termination")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("graceful\n")) {
		t.Fatalf("stderr = %q, want graceful signal acknowledgement", stderr.String())
	}
}

func TestTerminateAcceptsExitedProcess(t *testing.T) {
	cmd, _, _ := startTreeHelper(t, "exit")
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := Terminate(context.Background(), cmd.Process); err != nil {
		t.Fatal(err)
	}
}
