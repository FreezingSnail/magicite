package testenv

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAgentRecordsCallsAndEvents(t *testing.T) {
	env := New(t)
	fake := NewAgent(t, env, "kiro")
	if stdout, stderr, err := runAgent(env, "kiro", "chat", "--no-interactive", "--agent", "worker", "--model", "model", "--effort", "high", "--trust-all-tools", "plan"); err != nil || len(stderr) != 0 {
		t.Fatalf("complete = %q, %q, %v", stdout, stderr, err)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("Calls() = %#v", calls)
	}
	if got, want := calls[0].Argv[len(calls[0].Argv)-2:], []string{"--trust-all-tools", "plan"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv tail = %#v, want %#v", got, want)
	}
	lines := fake.Events("ses_completed")
	if len(lines) != 2 {
		t.Fatalf("Events() = %#v", lines)
	}
	for _, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("event %q is not JSON: %v", line, err)
		}
	}
}

func TestAgentScenarioForOverridesDefault(t *testing.T) {
	env := New(t)
	fake := NewAgent(t, env, "opencode")
	fake.Scenario("limited")
	fake.ScenarioFor("task-1", "complete")
	if _, stderr, err := runAgent(env, "opencode", "chat", "--model", "model", "--trust-all-tools", "work task-1 now"); err != nil || len(stderr) != 0 {
		t.Fatalf("task scenario = %q, %v", stderr, err)
	}
	if _, stderr, err := runAgent(env, "opencode", "chat", "--model", "model", "--trust-all-tools", "other task"); err != nil || len(stderr) != 0 {
		t.Fatalf("default scenario = %q, %v", stderr, err)
	}
	if got := fake.Events("ses_completed"); len(got) == 0 {
		t.Fatal("task-specific completion was not recorded")
	}
	if got := fake.Events("ses_limited"); len(got) == 0 {
		t.Fatal("default limit was not recorded")
	}
}

func TestAgentHangReportsChild(t *testing.T) {
	env := New(t)
	fake := NewAgent(t, env, "kiro")
	fake.Scenario("hang")
	command := exec.Command(env.Bin("kiro"), "chat", "--no-interactive", "--model", "model", "--trust-all-tools", "plan")
	command.Dir, command.Env = env.Root, env.Env()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var pids []int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pids = fake.ChildPIDs()
		if len(pids) == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(pids) != 1 {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("ChildPIDs() = %#v, want one pid", pids)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pids[0], 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("child %d survived hang termination", pids[0])
}

func runAgent(env *Env, name string, args ...string) ([]byte, []byte, error) {
	command := exec.Command(env.Bin(name), args...)
	command.Dir, command.Env = env.Root, env.Env()
	var stdout, stderr strings.Builder
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}

func TestAgentChildPIDsMissingIsEmpty(t *testing.T) {
	env := New(t)
	fake := NewAgent(t, env, "kiro")
	if got := fake.ChildPIDs(); len(got) != 0 {
		t.Errorf("ChildPIDs() = %#v, want empty", got)
	}
	if _, err := os.Stat(env.Bin("kiro")); err != nil {
		t.Fatal(err)
	}
}
