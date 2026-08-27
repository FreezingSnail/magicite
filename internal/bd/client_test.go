package bd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestNewDefaultsProgramAndRejectsInvalidRoot(t *testing.T) {
	root := t.TempDir()
	client, err := New("", root)
	if err != nil {
		t.Fatal(err)
	}
	if client.Program != "bd" || client.Root != root {
		t.Fatalf("client = %#v", client)
	}
	if _, err := New("bd", ""); err == nil {
		t.Error("New accepted empty root")
	}
	if _, err := New("bd", filepath.Join(root, "missing")); err == nil {
		t.Error("New accepted missing root")
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New("bd", file); err == nil {
		t.Error("New accepted file root")
	}
}

func TestRunPreservesArgvStreamsAndExitStatus(t *testing.T) {
	argument := `literal; $(not-a-command) && "quoted"`
	client := newFake(t, fakeEntry{
		Match:  []string{"ready", "--json"},
		Stdout: "out",
		Stderr: "err",
		Exit:   7,
	})

	result, err := client.Run(context.Background(), "ready", "--json", argument)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 7 || string(result.Stdout) != "out" || string(result.Stderr) != "err" {
		t.Fatalf("result = %+v", result)
	}
	want := [][]string{{"-C", client.Root, "ready", "--json", argument}}
	if got := fakeCalls(t, client); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %#v, want %#v", got, want)
	}
}

func TestRunReturnsErrorsForCancellationAndSpawnFailure(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"wait"}, DelayMS: 500})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := client.Run(ctx, "wait")
		result <- err
	}()
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after cancellation")
	}

	client, err := New(filepath.Join(t.TempDir(), "missing-bd"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Run(context.Background(), "ready"); err == nil {
		t.Error("Run accepted missing executable")
	}
}

func TestRunLogsOnlyReturnedFailureToClientLogger(t *testing.T) {
	var output bytes.Buffer
	client := newFake(t, fakeEntry{Match: []string{"exit"}, Exit: 2})
	client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})
	if _, err := client.Run(context.Background(), "exit"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output.Len() != 0 {
		t.Errorf("exit status logged: %s", output.String())
	}

	client.Program = filepath.Join(t.TempDir(), "missing-bd")
	if _, err := client.Run(context.Background(), "exit"); err == nil {
		t.Fatal("Run accepted missing executable")
	}
	var events []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'}) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1: %s", len(events), output.String())
	}
}
