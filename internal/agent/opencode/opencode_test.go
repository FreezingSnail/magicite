package opencode

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/FreezingSnail/magicite/internal/agent"
	"github.com/FreezingSnail/magicite/internal/agent/conformance"
)

func TestNew(t *testing.T) {
	if got := New(Options{}); got.Name() != "opencode" || got.Executable() != "opencode" {
		t.Fatalf("New default = (%q, %q), want (opencode, opencode)", got.Name(), got.Executable())
	}
	if got := New(Options{Executable: "custom-opencode"}); got.Executable() != "custom-opencode" {
		t.Fatalf("Executable() = %q", got.Executable())
	}
}

func TestValidEffort(t *testing.T) {
	for _, test := range []struct {
		effort string
		want   bool
	}{
		{"", false},
		{"high", true},
		{"experimental-v2", true},
		{"low effort", false},
		{"low\n", false},
		{"low/high", false},
	} {
		if got := ValidEffort(test.effort); got != test.want {
			t.Errorf("ValidEffort(%q) = %t, want %t", test.effort, got, test.want)
		}
	}
}

func TestRunArgs(t *testing.T) {
	got := RunArgs("/bin/opencode", "/work", "provider/model", "builder", "high", "opencode-1", "implement feature")
	want := []string{"/bin/opencode", "run", "--dir", "/work", "-m", "provider/model", "--variant", "high", "--agent", "builder", "--format", "json", "--auto", "--title", "opencode-1", "implement feature"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs() = %#v, want %#v", got, want)
	}
	got = RunArgs("opencode", "work", "model", "", "not valid", "opencode-2", "--plan")
	want = []string{"opencode", "run", "--dir", "work", "-m", "model", "--format", "json", "--auto", "--title", "opencode-2", "--plan"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs() omitted = %#v, want %#v", got, want)
	}
}

func TestRunMissingExecutableLeavesNoState(t *testing.T) {
	adapter := New(Options{Executable: "missing-opencode-" + time.Now().Format("150405.000000000")})
	_, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: t.TempDir(), Plan: "plan"})
	if !errors.Is(err, agent.ErrExecutableMissing) {
		t.Fatalf("Run() error = %v, want ErrExecutableMissing", err)
	}
	if handles := adapter.store.Handles(); len(handles) != 0 {
		t.Fatalf("failed Run left handles %v", handles)
	}
}

func TestOutputRawTranscriptAndLimitedStatus(t *testing.T) {
	adapter := New(Options{Executable: conformance.FakeCLI(t)})
	handle, err := adapter.Run(context.Background(), agent.RunSpec{
		Workdir: t.TempDir(), Model: "fake-model", Plan: conformance.ScenarioLimited,
	})
	if err != nil {
		t.Fatal(err)
	}
	await(t, adapter, handle, agent.StatusLimited)
	output, err := adapter.Output(context.Background(), handle)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(output) || output == "" {
		t.Fatalf("Output() = %q, want raw NDJSON", output)
	}
	if !adapter.UsageLimited(context.Background(), handle) {
		t.Fatal("UsageLimited() = false")
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
	t.Fatalf("Complete() = (%q, %v), want %q", got, err, want)
}
